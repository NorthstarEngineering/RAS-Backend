param(
    [Parameter(Mandatory = $true, Position = 0)]
    [ValidateSet('up', 'infra-up', 'ephemeral', 'down', 'down-preserve-data', 'shell-down')]
    [string]$Action,
    [string]$Profile = 'ras-backend-cloud',
    [string]$Region = 'eu-west-3',
    [switch]$UsePublishedImages,
    [switch]$AutoDestroy,
    [string]$ImageTag
)

$ErrorActionPreference = 'Stop'
if (-not $env:TF_DATA_DIR -and $env:RAS_TERRAFORM_DATA_DIR) { $env:TF_DATA_DIR = [System.IO.Path]::GetFullPath($env:RAS_TERRAFORM_DATA_DIR) }
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$terraformDirectory = Join-Path $repoRoot 'infra\terraform\dev'
$toolsDirectory = Join-Path $repoRoot '.tools\terraform'
$terraform = if ($env:RAS_TERRAFORM_BIN) {
    [System.IO.Path]::GetFullPath($env:RAS_TERRAFORM_BIN)
} else {
    Join-Path $toolsDirectory 'terraform.exe'
}
if (-not (Test-Path -LiteralPath $terraform)) {
    $terraformCommand = Get-Command terraform -ErrorAction SilentlyContinue
    if (-not $terraformCommand) { throw 'Terraform is required. Install Terraform or place terraform.exe in .tools\terraform.' }
    $terraform = $terraformCommand.Source
}

$secretFiles = @()
if ($env:RAS_TERRAFORM_SECRETS_FILE) {
    $secretFiles = @([System.IO.Path]::GetFullPath($env:RAS_TERRAFORM_SECRETS_FILE))
} else {
    $secretFiles = @(Get-ChildItem -LiteralPath $toolsDirectory -Filter '*-secrets.tfvars' -File -ErrorAction SilentlyContinue | ForEach-Object FullName)
}
if ($secretFiles.Count -ne 1 -or -not (Test-Path -LiteralPath $secretFiles[0])) {
    throw 'Set RAS_TERRAFORM_SECRETS_FILE or keep exactly one ignored *-secrets.tfvars file in .tools\terraform.'
}
$secretFile = $secretFiles[0]
$planPath = Join-Path $toolsDirectory "ras-backend-dev-$Action.tfplan"
$tfvarsPath = Join-Path $terraformDirectory 'terraform.tfvars'
if (Test-Path -LiteralPath $tfvarsPath) {
    $tfvarsContent = Get-Content -Raw -LiteralPath $tfvarsPath
    $tfvarsContent = [regex]::Replace($tfvarsContent, '(?m)^aws_profile\s*=.*$', 'aws_profile = ""')
    [System.IO.File]::WriteAllText($tfvarsPath, $tfvarsContent, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Terraform {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    Set-AwsSessionEnvironment
    & $terraform "-chdir=$terraformDirectory" @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Terraform failed: $($Arguments -join ' ')" }
}

function Invoke-Aws {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    Set-AwsSessionEnvironment
    & aws @Arguments --profile $Profile --region $Region
    if ($LASTEXITCODE -ne 0) { throw "AWS CLI failed: aws $($Arguments -join ' ')" }
}

function Set-AwsSessionEnvironment {
    $credentialOutput = & aws configure export-credentials --profile $Profile --format process
    if ($LASTEXITCODE -ne 0) { throw "Could not refresh AWS credentials from profile '$Profile'." }
    $session = ($credentialOutput | ConvertFrom-Json)
    if (-not $session.AccessKeyId -or -not $session.SecretAccessKey -or -not $session.SessionToken) {
        throw "AWS profile '$Profile' did not return a complete temporary credential session."
    }
    $env:AWS_ACCESS_KEY_ID = $session.AccessKeyId
    $env:AWS_SECRET_ACCESS_KEY = $session.SecretAccessKey
    $env:AWS_SESSION_TOKEN = $session.SessionToken
}

function Get-TerraformOutput {
    param([string]$Name)
    $value = & $terraform "-chdir=$terraformDirectory" output -raw $Name
    if ($LASTEXITCODE -ne 0) { throw "Missing Terraform output: $Name" }
    return $value.Trim()
}

function Get-StateBucket {
    $accountId = Invoke-Aws sts get-caller-identity --query Account --output text
    return "ras-backend-tfstate-$accountId-$Region"
}

function Remove-StateBucket {
    param([string]$Bucket)
    $versionsJson = Invoke-Aws s3api list-object-versions --bucket $Bucket --output json
    $versions = $versionsJson | ConvertFrom-Json
    foreach ($entry in @($versions.Versions) + @($versions.DeleteMarkers)) {
        if ($entry -and $entry.Key -and $entry.VersionId) {
            $null = Invoke-Aws s3api delete-object --bucket $Bucket --key $entry.Key --version-id $entry.VersionId
        }
    }
    $null = Invoke-Aws s3api delete-bucket --bucket $Bucket
}

function Remove-DevelopmentEcrImages {
    $repositories = @(
        'ras-auth-api', 'ras-branch-api', 'ras-attendance-api', 'ras-menu-api',
        'ras-central-kitchen-api', 'ras-inventory-api', 'ras-cost-api',
        'ras-sales-api', 'ras-transfer-api', 'ras-reports-api', 'ras-db-migrations'
    )
    $existingJson = Invoke-Aws ecr describe-repositories --query 'repositories[].repositoryName' --output json
    $existing = @($existingJson | ConvertFrom-Json)
    foreach ($repository in $repositories) {
        if ($repository -notin $existing) { continue }
        $imageJson = Invoke-Aws ecr describe-images --repository-name $repository --query 'imageDetails[].imageDigest' --output json
        $digests = @($imageJson | ConvertFrom-Json)
        for ($offset = 0; $offset -lt $digests.Count; $offset += 100) {
            $end = [Math]::Min($offset + 99, $digests.Count - 1)
            $imageIds = @($digests[$offset..$end] | ForEach-Object { "imageDigest=$_" })
            $resultJson = Invoke-Aws ecr batch-delete-image --repository-name $repository --image-ids $imageIds --output json
            $result = $resultJson | ConvertFrom-Json
            if (@($result.failures).Count -gt 0) {
                $reasons = @($result.failures | ForEach-Object { "$($_.imageId.imageDigest):$($_.failureCode)" }) -join ', '
                throw "Could not empty ECR repository $repository before teardown: $reasons"
            }
        }
    }
}

function Remove-OrphanedDevelopmentVpc {
    $vpcOutput = Invoke-Aws ec2 describe-vpcs --filters 'Name=tag:Project,Values=RAS-backend' 'Name=tag:Name,Values=ras-backend-dev-vpc' --query 'Vpcs[].VpcId' --output text
    foreach ($vpcId in @($vpcOutput -split '\s+' | Where-Object { $_ -match '^vpc-' })) {
        $endpointJson = Invoke-Aws ec2 describe-vpc-endpoints --filters "Name=vpc-id,Values=$vpcId" --query 'VpcEndpoints[].VpcEndpointId' --output text
        $endpointIds = @($endpointJson -split '\s+' | Where-Object { $_ -match '^vpce-' })
        if ($endpointIds.Count -gt 0) { $null = Invoke-Aws ec2 delete-vpc-endpoints --vpc-endpoint-ids $endpointIds }
        $natJson = Invoke-Aws ec2 describe-nat-gateways --filter "Name=vpc-id,Values=$vpcId" 'Name=state,Values=pending,available,failed' --query 'NatGateways[].NatGatewayId' --output text
        foreach ($natId in @($natJson -split '\s+' | Where-Object { $_ -match '^nat-' })) { $null = Invoke-Aws ec2 delete-nat-gateway --nat-gateway-id $natId }
        $instancesJson = Invoke-Aws ec2 describe-instances --filters "Name=vpc-id,Values=$vpcId" 'Name=instance-state-name,Values=pending,running,stopping,stopped' --query 'Reservations[].Instances[].InstanceId' --output text
        $instanceIds = @($instancesJson -split '\s+' | Where-Object { $_ -match '^i-' })
        if ($instanceIds.Count -gt 0) {
            $null = Invoke-Aws ec2 terminate-instances --instance-ids $instanceIds
            $null = Invoke-Aws ec2 wait instance-terminated --instance-ids $instanceIds
        }
        $functionsJson = Invoke-Aws lambda list-functions --query 'Functions[?starts_with(FunctionName, `ras-backend-dev-`)].FunctionName' --output text
        foreach ($functionName in @($functionsJson -split '\s+' | Where-Object { $_ })) { $null = Invoke-Aws lambda delete-function --function-name $functionName }
        $eniJson = Invoke-Aws ec2 describe-network-interfaces --filters "Name=vpc-id,Values=$vpcId" --query 'NetworkInterfaces[].{Id:NetworkInterfaceId,Status:Status}' --output json
        foreach ($eni in @($eniJson | ConvertFrom-Json)) {
            if ($eni.Status -eq 'available') { $null = Invoke-Aws ec2 delete-network-interface --network-interface-id $eni.Id }
            else { throw "Cannot remove residual development VPC $vpcId while network interface $($eni.Id) remains $($eni.Status)." }
        }
        $subnetsJson = Invoke-Aws ec2 describe-subnets --filters "Name=vpc-id,Values=$vpcId" --query 'Subnets[].SubnetId' --output text
        foreach ($subnetId in @($subnetsJson -split '\s+' | Where-Object { $_ -match '^subnet-' })) { $null = Invoke-Aws ec2 delete-subnet --subnet-id $subnetId }
        $gatewaysJson = Invoke-Aws ec2 describe-internet-gateways --filters "Name=attachment.vpc-id,Values=$vpcId" --query 'InternetGateways[].InternetGatewayId' --output text
        foreach ($gatewayId in @($gatewaysJson -split '\s+' | Where-Object { $_ -match '^igw-' })) {
            $null = Invoke-Aws ec2 detach-internet-gateway --internet-gateway-id $gatewayId --vpc-id $vpcId
            $null = Invoke-Aws ec2 delete-internet-gateway --internet-gateway-id $gatewayId
        }
        $groupsJson = Invoke-Aws ec2 describe-security-groups --filters "Name=vpc-id,Values=$vpcId" --query 'SecurityGroups[?GroupName!=`default`].GroupId' --output text
        $groupIds = @($groupsJson -split '\s+' | Where-Object { $_ -match '^sg-' })
        foreach ($groupId in $groupIds) {
            $rulesJson = Invoke-Aws ec2 describe-security-group-rules --filters "Name=group-id,Values=$groupId" --query 'SecurityGroupRules[].{Id:SecurityGroupRuleId,Egress:IsEgress}' --output json
            foreach ($rule in @($rulesJson | ConvertFrom-Json)) {
                $action = if ($rule.Egress) { 'revoke-security-group-egress' } else { 'revoke-security-group-ingress' }
                $null = Invoke-Aws ec2 $action --group-id $groupId --security-group-rule-ids $rule.Id
            }
        }
        foreach ($groupId in $groupIds) { $null = Invoke-Aws ec2 delete-security-group --group-id $groupId }
        $routeJson = Invoke-Aws ec2 describe-route-tables --filters "Name=vpc-id,Values=$vpcId" --query 'RouteTables[?Associations[?Main!=`true`]].RouteTableId' --output text
        foreach ($routeId in @($routeJson -split '\s+' | Where-Object { $_ -match '^rtb-' })) {
            $associationJson = Invoke-Aws ec2 describe-route-tables --route-table-ids $routeId --query 'RouteTables[0].Associations[?Main!=`true`].RouteTableAssociationId' --output text
            foreach ($associationId in @($associationJson -split '\s+' | Where-Object { $_ -match '^rtbassoc-' })) { $null = Invoke-Aws ec2 disassociate-route-table --association-id $associationId }
            $null = Invoke-Aws ec2 delete-route-table --route-table-id $routeId
        }
        $null = Invoke-Aws ec2 delete-vpc --vpc-id $vpcId
    }
}

function New-ReviewedPlan {
    $stateBucket = Get-StateBucket
    $bucketExists = $true
    try { $null = Invoke-Aws s3api head-bucket --bucket $stateBucket }
    catch { $bucketExists = $false }
    if (-not $bucketExists) {
        $location = if ($Region -eq 'us-east-1') { @{} } else { @{ LocationConstraint = $Region } }
        $locationJson = $location | ConvertTo-Json -Compress
        $requestPath = Join-Path $env:TEMP ("ras-state-bucket-" + [guid]::NewGuid().ToString('N') + '.json')
        [System.IO.File]::WriteAllText($requestPath, $locationJson, [System.Text.UTF8Encoding]::new($false))
        try { $null = Invoke-Aws s3api create-bucket --bucket $stateBucket --create-bucket-configuration "file://$requestPath" }
        finally { Remove-Item -LiteralPath $requestPath -Force -ErrorAction SilentlyContinue }
        $null = Invoke-Aws s3api put-public-access-block --bucket $stateBucket --public-access-block-configuration BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true
        $null = Invoke-Aws s3api put-bucket-versioning --bucket $stateBucket --versioning-configuration Status=Enabled
        $null = Invoke-Aws s3api put-bucket-encryption --bucket $stateBucket --server-side-encryption-configuration '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
        $null = Invoke-Aws s3api put-bucket-tagging --bucket $stateBucket --tagging "TagSet=[{Key=Project,Value=RAS-backend},{Key=Purpose,Value=terraform-state},{Key=Environment,Value=development}]"
        Start-Sleep -Seconds 3
    }
    $null = Invoke-Terraform init -input=false "-backend-config=bucket=$stateBucket" "-backend-config=key=ras-backend/dev/terraform.tfstate" "-backend-config=region=$Region"
    if ($Action -eq 'down') {
        $null = Invoke-Terraform plan -destroy -input=false "-var-file=terraform.tfvars" "-var-file=$secretFile" "-var=protect_database=false" "-out=$planPath"
    } elseif ($Action -eq 'down-preserve-data') {
        $targets = @(Get-PreserveDataTargets)
        if ($targets.Count -eq 0) { throw 'No disposable Terraform resources were found to destroy.' }
        $targetArguments = @($targets | ForEach-Object { "-target=$_" })
        $planArguments = @(
            'plan',
            '-destroy',
            '-input=false',
            '-var-file=terraform.tfvars',
            "-var-file=$secretFile",
            '-var=protect_database=true'
        ) + $targetArguments + @("-out=$planPath")
        $null = Invoke-Terraform @planArguments
    } elseif ($Action -eq 'shell-down') {
        $null = Invoke-Terraform plan -destroy -input=false "-var-file=terraform.tfvars" "-var-file=$secretFile" "-var=protect_database=true" "-target=module.application" "-out=$planPath"
    } else {
        $null = Invoke-Terraform plan -input=false "-var-file=terraform.tfvars" "-var-file=$secretFile" "-out=$planPath"
    }
    $planJson = (& $terraform "-chdir=$terraformDirectory" show -json $planPath | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect the Terraform plan.' }
    $changes = @($planJson.resource_changes | Where-Object { ($_.change.actions -join ',') -ne 'no-op' })
    if ($Action -eq 'up') {
        $deletions = @($changes | Where-Object { $_.change.actions -contains 'delete' })
        if ($deletions.Count -gt 0) {
            $addresses = $deletions | ForEach-Object address
            throw "Refusing an up plan that deletes or replaces resources: $($addresses -join ', ')"
        }
    }
    return $changes
}

function Get-PreserveDataTargets {
    $stateAddresses = @(& $terraform "-chdir=$terraformDirectory" state list)
    if ($LASTEXITCODE -ne 0) { throw 'Could not list Terraform state resources.' }
    return @(
        $stateAddresses |
            Where-Object {
                $_ -and
                $_ -notlike 'module.database.*' -and
                $_ -notlike 'module.network.*'
            }
    )
}

function Invoke-Ssm {
    param([string]$InstanceId, [string]$Command)
    $request = @{
        InstanceIds  = @($InstanceId)
        DocumentName = 'AWS-RunShellScript'
        Parameters   = @{ commands = @($Command) }
    } | ConvertTo-Json -Depth 6 -Compress
    $requestPath = Join-Path $env:TEMP ("ras-ssm-" + [guid]::NewGuid().ToString('N') + '.json')
    [System.IO.File]::WriteAllText($requestPath, $request, [System.Text.UTF8Encoding]::new($false))
    try {
        $commandId = Invoke-Aws ssm send-command --cli-input-json "file://$requestPath" --query 'Command.CommandId' --output text
    } finally {
        Remove-Item -LiteralPath $requestPath -Force -ErrorAction SilentlyContinue
    }
    $deadline = [DateTime]::UtcNow.AddMinutes(15)
    do {
        Start-Sleep -Seconds 8
        $status = Invoke-Aws ssm get-command-invocation --command-id $commandId --instance-id $InstanceId --query Status --output text
    } while ($status -in @('Pending', 'InProgress', 'Delayed') -and [DateTime]::UtcNow -lt $deadline)
    if ($status -ne 'Success') { throw "SSM command $commandId ended with status $status. Inspect it in Systems Manager Run Command." }
    $output = Invoke-Aws ssm get-command-invocation --command-id $commandId --instance-id $InstanceId --query StandardOutputContent --output text
    if ($output -and $output -ne 'None') { Write-Output $output.Trim() }
}

function ConvertTo-CompressedBase64 {
    param([string]$Content)
    $inputBytes = [System.Text.Encoding]::UTF8.GetBytes($Content)
    $outputStream = [System.IO.MemoryStream]::new()
    $gzipStream = [System.IO.Compression.GzipStream]::new(
        $outputStream,
        [System.IO.Compression.CompressionLevel]::Optimal,
        $true
    )
    try { $gzipStream.Write($inputBytes, 0, $inputBytes.Length) }
    finally { $gzipStream.Dispose() }
    try { return [Convert]::ToBase64String($outputStream.ToArray()) }
    finally { $outputStream.Dispose() }
}

function Sync-HostDeploymentHelper {
    param([string]$InstanceId)
    $repositoryJson = & $terraform "-chdir=$terraformDirectory" output -json ecr_repository_urls
    if ($LASTEXITCODE -ne 0) { throw 'Could not read deployed ECR repository URLs.' }
    $repositories = $repositoryJson | ConvertFrom-Json -AsHashtable
    $deployEnvironment = @(
        "AWS_REGION=$Region",
        'APP_SECRET_PARAMETER_NAME=/ras-backend/dev/application-secrets',
        "DATABASE_MASTER_PASSWORD_PARAMETER_NAME=$(Get-TerraformOutput 'database_master_password_parameter_name')",
        "DATABASE_MASTER_USERNAME=$(Get-TerraformOutput 'database_master_username')",
        "DATABASE_ENDPOINT=$(Get-TerraformOutput 'database_endpoint')",
        "DB_NAME=$(Get-TerraformOutput 'database_name')",
        "AUTH_IMAGE_REPOSITORY=$($repositories['ras-auth-api'])",
        "BRANCH_IMAGE_REPOSITORY=$($repositories['ras-branch-api'])",
        "ATTENDANCE_IMAGE_REPOSITORY=$($repositories['ras-attendance-api'])",
        "MENU_IMAGE_REPOSITORY=$($repositories['ras-menu-api'])",
        "CENTRAL_KITCHEN_IMAGE_REPOSITORY=$($repositories['ras-central-kitchen-api'])",
        "INVENTORY_IMAGE_REPOSITORY=$($repositories['ras-inventory-api'])",
        "COST_IMAGE_REPOSITORY=$($repositories['ras-cost-api'])",
        "SALES_IMAGE_REPOSITORY=$($repositories['ras-sales-api'])",
        "TRANSFER_IMAGE_REPOSITORY=$($repositories['ras-transfer-api'])",
        "REPORTS_IMAGE_REPOSITORY=$($repositories['ras-reports-api'])",
        "MIGRATION_IMAGE_REPOSITORY=$($repositories['ras-db-migrations'])"
    ) -join "`n"
    $deployScript = Get-Content -Raw -LiteralPath (Join-Path $repoRoot 'deploy\aws-dev-deploy.sh')
    $scriptPayload = ConvertTo-CompressedBase64 $deployScript
    $environmentPayload = ConvertTo-CompressedBase64 $deployEnvironment
    $command = "echo '$scriptPayload' | base64 -d | gzip -dc | sudo tee /usr/local/bin/ras-backend-deploy.sh >/dev/null && sudo chmod 0700 /usr/local/bin/ras-backend-deploy.sh && echo '$environmentPayload' | base64 -d | gzip -dc | sudo tee /etc/ras-backend-deploy.env >/dev/null && sudo chmod 0644 /etc/ras-backend-deploy.env"
    Invoke-Ssm $InstanceId $command
}

function Invoke-GitHubDevelopmentDeploy {
    $dispatchTime = [DateTime]::UtcNow
    $mainSha = (gh api repos/NorthstarEngineering/RAS-Backend/branches/main --jq '.commit.sha').Trim()
    if ($LASTEXITCODE -ne 0 -or $mainSha -notmatch '^[a-f0-9]{40}$') { throw 'Could not identify the current main commit for the deployment workflow.' }
    $null = gh workflow run 'Deploy AWS development stack' --ref main --repo NorthstarEngineering/RAS-Backend
    if ($LASTEXITCODE -ne 0) { throw 'Could not dispatch the main-branch AWS build, publish, and endpoint workflow.' }
    $deadline = [DateTime]::UtcNow.AddMinutes(5)
    $runId = $null
    do {
        Start-Sleep -Seconds 5
        $runsJson = gh run list --workflow deploy-development.yml --event workflow_dispatch --branch main --repo NorthstarEngineering/RAS-Backend --limit 10 --json databaseId,status,createdAt,headSha
        if ($LASTEXITCODE -ne 0) { throw 'Could not locate the dispatched AWS deployment workflow.' }
        $runs = @($runsJson | ConvertFrom-Json)
        $runId = ($runs | Where-Object { $_.headSha -eq $mainSha -and [DateTime]$_.createdAt -ge $dispatchTime.AddSeconds(-10) } | Select-Object -First 1).databaseId
    } while (-not $runId -and [DateTime]::UtcNow -lt $deadline)
    if (-not $runId) { throw 'The AWS deployment workflow did not start within five minutes.' }
    gh run watch $runId --repo NorthstarEngineering/RAS-Backend --exit-status | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "AWS build, publish, deploy, or endpoint workflow $runId failed. Inspect it with gh run view $runId." }
    return $mainSha
}

if ($Action -in @('up', 'infra-up')) {
    $dirty = @(git -C $repoRoot status --porcelain -- services database deploy/aws-dev-deploy.sh infra/terraform)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Git status.' }
    if ($dirty.Count -gt 0) { throw 'Commit or merge backend, migration, deployment, and Terraform changes before using this repeatable deploy command.' }
    $imageTag = if ($ImageTag) { $ImageTag } else { (git -C $repoRoot rev-parse HEAD).Trim() }
    if ($LASTEXITCODE -ne 0 -or $imageTag -notmatch '^[a-f0-9]{40}$') { throw 'Could not determine the source commit SHA.' }
    $secretsRaw = Get-Content -Raw -LiteralPath $secretFile
    $appSecretsEncoded = [regex]::Match($secretsRaw, '(?m)^app_secret_json\s*=\s*(.+)$').Groups[1].Value.Trim()
    if (-not $appSecretsEncoded) { throw 'The secrets tfvars file has no app_secret_json value.' }
    $appSecrets = ConvertFrom-Json (ConvertFrom-Json $appSecretsEncoded)
    if (-not $appSecrets.REPORTS_DB_PASSWORD) {
        $appSecrets | Add-Member -NotePropertyName REPORTS_DB_PASSWORD -NotePropertyValue ([guid]::NewGuid().ToString('N') + [guid]::NewGuid().ToString('N'))
        $appJson = ConvertTo-Json -InputObject $appSecrets -Compress
        $masterPassword = [regex]::Match($secretsRaw, '(?m)^database_master_password\s*=\s*(.+)$').Groups[1].Value
        $secretText = "database_master_password = $masterPassword`napp_secret_json = $(ConvertTo-Json -InputObject $appJson -Compress)`n"
        [System.IO.File]::WriteAllText($secretFile, $secretText, [System.Text.UTF8Encoding]::new($false))
    }
}

if ($Action -eq 'infra-up') {
    $changes = New-ReviewedPlan
    if ($changes.Count -gt 0) {
        Write-Output 'Applying the reviewed non-destructive development infrastructure plan.'
        Invoke-Terraform apply -input=false -auto-approve $planPath
    } else { Write-Output 'Infrastructure is already current.' }
    $environmentName = 'development-publish'
    $roleArn = Get-TerraformOutput 'github_actions_publish_role_arn'
    $regionValue = Get-TerraformOutput 'github_actions_aws_region'
    $gatewayBaseUrl = Get-TerraformOutput 'github_actions_api_gateway_base_url'
    $null = gh auth refresh -s repo,workflow
    if ($LASTEXITCODE -ne 0) { throw 'GitHub CLI needs repo/workflow administration authorization to configure OIDC deployment.' }
    $environmentUri = "repos/NorthstarEngineering/RAS-Backend/environments/$environmentName"
    $null = gh api --method PUT $environmentUri --field wait_timer=0 --field can_admins_bypass=false
    if ($LASTEXITCODE -ne 0) { throw "Could not create/configure GitHub environment $environmentName." }
    gh variable set AWS_ROLE_TO_ASSUME --repo NorthstarEngineering/RAS-Backend --env $environmentName --body $roleArn
    if ($LASTEXITCODE -ne 0) { throw 'Could not set AWS_ROLE_TO_ASSUME in GitHub Actions environment.' }
    gh variable set AWS_REGION --repo NorthstarEngineering/RAS-Backend --env $environmentName --body $regionValue
    if ($LASTEXITCODE -ne 0) { throw 'Could not set AWS_REGION in GitHub Actions environment.' }
    gh variable set API_GATEWAY_BASE_URL --repo NorthstarEngineering/RAS-Backend --env $environmentName --body $gatewayBaseUrl
    if ($LASTEXITCODE -ne 0) { throw 'Could not set API_GATEWAY_BASE_URL in GitHub Actions environment.' }
    Write-Output 'Infrastructure is ready. Publish images with the Docker stack workflow, then run aws-dev.ps1 up -UsePublishedImages.'
    Write-Output "GitHub Actions environment '$environmentName' has been configured for OIDC publishing and deployment."
    exit 0
}

if ($Action -eq 'up') {
    if (-not $UsePublishedImages) {
        if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Docker is required. Start Docker Desktop, then rerun this command.' }
        docker info --format '{{.ServerVersion}}' | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Docker Engine is not running.' }
    }

    $changes = New-ReviewedPlan
    if ($changes.Count -gt 0) {
        Write-Output 'Applying the reviewed non-destructive development plan.'
        Invoke-Terraform apply -input=false -auto-approve $planPath
    } else { Write-Output 'Infrastructure is already current.' }

    $account = Invoke-Aws sts get-caller-identity --query Account --output text
    $registry = "$account.dkr.ecr.$Region.amazonaws.com"
    $images = @(
        @{ Name = 'ras-auth-api'; Context = 'services\auth-api'; Dockerfile = 'services\auth-api\Dockerfile' },
        @{ Name = 'ras-branch-api'; Context = 'services\branch-api'; Dockerfile = 'services\branch-api\Dockerfile' },
        @{ Name = 'ras-attendance-api'; Context = 'services\attendance-api'; Dockerfile = 'services\attendance-api\Dockerfile' },
        @{ Name = 'ras-menu-api'; Context = 'services\menu-api'; Dockerfile = 'services\menu-api\Dockerfile' },
        @{ Name = 'ras-central-kitchen-api'; Context = 'services\central-kitchen-supply'; Dockerfile = 'services\central-kitchen-supply\Dockerfile' },
        @{ Name = 'ras-inventory-api'; Context = 'services\inventory-api'; Dockerfile = 'services\inventory-api\Dockerfile' },
        @{ Name = 'ras-cost-api'; Context = 'services\cost-api'; Dockerfile = 'services\cost-api\Dockerfile' },
        @{ Name = 'ras-sales-api'; Context = 'services\sales-api'; Dockerfile = 'services\sales-api\Dockerfile' },
        @{ Name = 'ras-transfer-api'; Context = 'services\transfer-api'; Dockerfile = 'services\transfer-api\Dockerfile' },
        @{ Name = 'ras-reports-api'; Context = 'services\reports-api'; Dockerfile = 'services\reports-api\Dockerfile' },
        @{ Name = 'ras-db-migrations'; Context = '.'; Dockerfile = 'database\Dockerfile' }
    )
    if ($UsePublishedImages) {
        foreach ($image in $images) {
            $digest = Invoke-Aws ecr describe-images --repository-name $image.Name --image-ids "imageTag=$imageTag" --query 'imageDetails[0].imageDigest' --output text
            if (-not $digest -or $digest -eq 'None') { throw "Immutable ECR image is missing for $($image.Name):$imageTag. Run the Docker stack workflow on main first." }
        }
    } else {
        $login = Invoke-Aws ecr get-login-password
        $login | docker login --username AWS --password-stdin $registry | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Docker could not authenticate to ECR.' }
        foreach ($image in $images) {
            $localTag = "$($image.Name):$imageTag"
            $remoteTag = "$registry/$($image.Name):$imageTag"
            docker build -f (Join-Path $repoRoot $image.Dockerfile) -t $localTag (Join-Path $repoRoot $image.Context)
            if ($LASTEXITCODE -ne 0) { throw "Docker build failed for $($image.Name)." }
            docker tag $localTag $remoteTag
            if ($LASTEXITCODE -ne 0) { throw "Docker tag failed for $($image.Name)." }
            docker push $remoteTag
            if ($LASTEXITCODE -ne 0) { throw "ECR push failed for $($image.Name)." }
        }
    }

    $instanceId = Get-TerraformOutput 'application_instance_id'
    $gatewayUrl = Get-TerraformOutput 'api_gateway_base_url'
    $deadline = [DateTime]::UtcNow.AddMinutes(15)
    do {
        Start-Sleep -Seconds 10
        $ping = Invoke-Aws ssm describe-instance-information --filters "Key=InstanceIds,Values=$instanceId" --query 'InstanceInformationList[0].PingStatus' --output text
    } while ($ping -ne 'Online' -and [DateTime]::UtcNow -lt $deadline)
    if ($ping -ne 'Online') { throw "EC2 host $instanceId did not become SSM-online." }
    Sync-HostDeploymentHelper $instanceId
    Invoke-Ssm $instanceId 'sudo /usr/local/bin/ras-host-doctor.sh'
    Invoke-Ssm $instanceId "sudo /usr/local/bin/ras-backend-deploy.sh migrate $imageTag"
    foreach ($service in @('auth-api', 'branch-api', 'attendance-api', 'menu-api', 'central-kitchen-api', 'inventory-api', 'cost-api', 'sales-api', 'transfer-api', 'reports-api')) {
        Invoke-Ssm $instanceId "sudo /usr/local/bin/ras-backend-deploy.sh $service $imageTag"
    }
    $endpointChecks = @(
        @{ Name = 'gateway/auth'; Path = '/health'; Port = 9001; LocalPath = '/health' },
        @{ Name = 'auth'; Path = '/api/v1/auth/health'; Port = 9001; LocalPath = '/health' },
        @{ Name = 'branch'; Path = '/api/v1/branches/health'; Port = 9002; LocalPath = '/health' },
        @{ Name = 'attendance'; Path = '/api/v1/attendance/health'; Port = 9003; LocalPath = '/health' },
        @{ Name = 'central-kitchen'; Path = '/api/v1/central-kitchen/health'; Port = 9004; LocalPath = '/health' },
        @{ Name = 'menu'; Path = '/api/v1/menu/health'; Port = 9005; LocalPath = '/health' },
        @{ Name = 'inventory'; Path = '/api/v1/stock/health'; Port = 9010; LocalPath = '/healthz' },
        @{ Name = 'sales'; Path = '/api/v1/sales/health'; Port = 9006; LocalPath = '/health' },
        @{ Name = 'cost'; Path = '/api/v1/cost/health'; Port = 9007; LocalPath = '/health' },
        @{ Name = 'transfer'; Path = '/api/v1/transfers/health'; Port = 9008; LocalPath = '/healthz' },
        @{ Name = 'reports'; Path = '/api/v1/reports/health'; Port = 9009; LocalPath = '/health' }
    )
    $endpointResults = @()
    foreach ($check in $endpointChecks) {
        $uri = "$gatewayUrl$($check.Path)"
        $targetUri = "http://127.0.0.1:$($check.Port)$($check.LocalPath)"
        if (-not $isGatewayHealth) {
            try {
                Invoke-Ssm $instanceId "curl --fail --silent --show-error --max-time 10 '$targetUri' >/dev/null"
                $endpointResults += [pscustomobject]@{ Name = "$($check.Name)/host"; Path = $check.LocalPath; Status = 200; Result = 'PASS' }
            } catch {
                $endpointResults += [pscustomobject]@{ Name = "$($check.Name)/host"; Path = $check.LocalPath; Status = 0; Result = 'FAIL' }
            }
        }
        try {
            $response = Invoke-WebRequest -Uri $uri -TimeoutSec 25 -UseBasicParsing
            $endpointResults += [pscustomobject]@{ Name = $check.Name; Path = $check.Path; Status = [int]$response.StatusCode; Result = if ([int]$response.StatusCode -eq 200) { 'PASS' } else { 'FAIL' } }
        } catch {
            $status = if ($_.Exception.Response) { [int]$_.Exception.Response.StatusCode } else { 0 }
            $endpointResults += [pscustomobject]@{ Name = $check.Name; Path = $check.Path; Status = $status; Result = 'FAIL' }
        }
    }
    $endpointResults | Format-Table -AutoSize | Out-String | Write-Output
    $failedEndpoints = @($endpointResults | Where-Object Result -ne 'PASS')
    $reportPath = Join-Path $repoRoot 'docs\aws-cloud-deployment-report.md'
    $reportLines = @(
        '# AWS Cloud Deployment and Endpoint Verification',
        '',
        "- Run time (UTC): $([DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ'))",
        "- Source commit: $imageTag",
        "- Region: $Region",
        "- API Gateway: $gatewayUrl",
        '- Deployment test type: immutable Docker image publish, SSM migration/deployment, private and public HTTP health smoke.',
        '',
        '| Endpoint | HTTP | Result |',
        '|---|---:|---|'
    )
    $reportLines += @($endpointResults | ForEach-Object { "| $($_.Name) ($($_.Path)) | $($_.Status) | $($_.Result) |" })
    [System.IO.File]::WriteAllLines($reportPath, $reportLines, [System.Text.UTF8Encoding]::new($false))
    if ($failedEndpoints.Count -gt 0) {
        Add-Content -LiteralPath $reportPath -Value "`nResult: FAIL. Endpoint verification failed; see the failed rows above."
        $failedNames = $failedEndpoints | ForEach-Object { "$($_.Name) ($($_.Path)): HTTP $($_.Status)" }
        throw "Development deploy endpoint checks failed: $($failedNames -join '; ')"
    }
    Add-Content -LiteralPath $reportPath -Value "`nResult: PASS. All private service and public gateway health endpoints returned HTTP 200."
    Write-Output "Development APIs are available at $gatewayUrl; endpoint report: $reportPath"
}
elseif ($Action -eq 'ephemeral') {
    $reportPath = Join-Path $repoRoot 'docs\aws-cloud-deployment-report.md'
    $deployOutcome = 'FAIL'
    $deployError = $null
    $destroyOutcome = 'FAIL'
    try {
        & $PSCommandPath -Action down -Profile $Profile -Region $Region -AutoDestroy
        if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw "Initial cleanup exited with code $LASTEXITCODE." }
        Remove-OrphanedDevelopmentVpc
        Remove-StateBucket (Get-StateBucket)
        Remove-Item -LiteralPath (Join-Path $toolsDirectory 'ras-backend-dev-down.tfplan') -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'RAS-Backend\terraform-state-backups\*.state.dpapi') -Force -ErrorAction SilentlyContinue
        & $PSCommandPath -Action infra-up -Profile $Profile -Region $Region
        if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw "Infrastructure bootstrap exited with code $LASTEXITCODE." }
        $publishedImageTag = Invoke-GitHubDevelopmentDeploy
        & $PSCommandPath -Action up -Profile $Profile -Region $Region -UsePublishedImages -ImageTag $publishedImageTag
        if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw "Deployment command exited with code $LASTEXITCODE." }
        $deployOutcome = 'PASS'
    } catch {
        $deployError = $_.Exception.Message
    } finally {
        try {
            & $PSCommandPath -Action down -Profile $Profile -Region $Region -AutoDestroy
            if ($LASTEXITCODE -and $LASTEXITCODE -ne 0) { throw "Teardown command exited with code $LASTEXITCODE." }
            $destroyOutcome = 'PASS'
            $bucket = Get-StateBucket
            Remove-OrphanedDevelopmentVpc
            Remove-StateBucket $bucket
            Remove-Item -LiteralPath (Join-Path $toolsDirectory 'ras-backend-dev-down.tfplan') -Force -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath (Join-Path $env:LOCALAPPDATA 'RAS-Backend\terraform-state-backups\*.state.dpapi') -Force -ErrorAction SilentlyContinue
        } catch {
            $destroyOutcome = "FAIL: $($_.Exception.Message)"
        }
    }
    if (-not (Test-Path -LiteralPath $reportPath)) {
        [System.IO.File]::WriteAllText($reportPath, "# AWS Cloud Deployment and Endpoint Verification`n`nNo endpoint run completed.`n", [System.Text.UTF8Encoding]::new($false))
    }
    Add-Content -LiteralPath $reportPath -Value "`n## Ephemeral lifecycle`n`n- Deployment: $deployOutcome`n- Teardown, including RDS data and versioned S3 state bucket: $destroyOutcome`n"
    if ($deployError) { Add-Content -LiteralPath $reportPath -Value "- Deployment error: $deployError`n" }
    if ($destroyOutcome -ne 'PASS') { throw "Cloud teardown did not complete: $destroyOutcome. Inspect the Terraform state bucket and rerun cleanup after AWS authentication is restored." }
    if ($deployOutcome -ne 'PASS') { throw "Cloud deployment failed and the cloud stack was destroyed. Details are in $reportPath. $deployError" }
    Write-Output "Ephemeral cloud deployment, endpoint smoke, report, and full AWS teardown completed: $reportPath"
}
elseif ($Action -eq 'shell-down') {
    $changes = New-ReviewedPlan
    $addresses = @($changes | ForEach-Object address)
    Write-Output "The following shell resources will be destroyed: $($addresses -join ', ')"
    $confirmation = Read-Host 'Type DELETE RAS BACKEND DEV SHELL to continue'
    if ($confirmation -cne 'DELETE RAS BACKEND DEV SHELL') { throw 'Shell teardown cancelled; no resources were destroyed.' }
    Invoke-Terraform apply -input=false -auto-approve $planPath
    Write-Output 'The EC2 shell was destroyed. RDS, its data, and the Terraform state bucket were retained.'
}
elseif ($Action -eq 'down-preserve-data') {
    $changes = New-ReviewedPlan
    $addresses = @($changes | ForEach-Object address)
    Write-Output "The following disposable resources will be destroyed: $($addresses -join ', ')"
    Write-Warning 'RDS, its database subnet/security group, VPC networking, and Terraform state bucket will be retained. EC2, APIs, Lambdas, ECR, IAM application resources, and SSM parameters will be destroyed.'
    $confirmation = Read-Host 'Type DELETE RAS BACKEND DEV APPLICATION to continue'
    if ($confirmation -cne 'DELETE RAS BACKEND DEV APPLICATION') { throw 'Application teardown cancelled; no resources were destroyed.' }
    Invoke-Terraform apply -input=false -auto-approve $planPath
    Write-Output 'Disposable application infrastructure destroyed. RDS data and required networking were retained.'
}
else {
    $stateBucket = Get-StateBucket
    Invoke-Terraform init -input=false "-backend-config=bucket=$stateBucket" "-backend-config=key=ras-backend/dev/terraform.tfstate" "-backend-config=region=$Region"
    $state = (& $terraform "-chdir=$terraformDirectory" state pull | Out-String)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($state)) { throw 'Could not back up remote Terraform state; refusing teardown.' }
    $null = $state | ConvertFrom-Json -ErrorAction Stop
    $backupDirectory = Join-Path $env:LOCALAPPDATA 'RAS-Backend\terraform-state-backups'
    New-Item -ItemType Directory -Path $backupDirectory -Force | Out-Null
    $plainBytes = [System.Text.Encoding]::UTF8.GetBytes($state)
    Add-Type -AssemblyName System.Security
    $encryptedState = [System.Security.Cryptography.ProtectedData]::Protect(
        $plainBytes,
        $null,
        [System.Security.Cryptography.DataProtectionScope]::CurrentUser
    )
    $backupPath = Join-Path $backupDirectory ("ras-backend-dev-" + [DateTime]::UtcNow.ToString('yyyyMMddTHHmmssZ') + '.state.dpapi')
    [System.IO.File]::WriteAllBytes($backupPath, $encryptedState)
    Write-Output "Encrypted local state backup saved at $backupPath"

    $changes = New-ReviewedPlan
    $addresses = @($changes | ForEach-Object address)
    Write-Output "The following AWS resources will be destroyed: $($addresses -join ', ')"
    Write-Warning 'This deletes the development RDS database and its data. The separately managed, versioned Terraform state S3 bucket is retained.'
    $confirmation = if ($AutoDestroy) { 'DELETE RAS BACKEND DEV DATA' } else { Read-Host 'Type DELETE RAS BACKEND DEV DATA to continue' }
    if ($confirmation -cne 'DELETE RAS BACKEND DEV DATA') { throw 'Teardown cancelled; no resources were destroyed.' }
    Remove-DevelopmentEcrImages
    Invoke-Terraform apply -input=false -auto-approve $planPath
    Write-Output 'Development infrastructure destroyed. Terraform state bucket and encrypted laptop backup remain.'
}
