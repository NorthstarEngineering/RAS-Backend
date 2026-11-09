param(
    [Parameter(Mandatory = $true, Position = 0)]
    [ValidateSet('up', 'down', 'down-preserve-data', 'shell-down')]
    [string]$Action,
    [string]$Profile = 'terraform',
    [string]$Region = 'eu-west-3'
)

$ErrorActionPreference = 'Stop'
$repoRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$terraformDirectory = Join-Path $repoRoot 'infra\terraform\dev'
$toolsDirectory = Join-Path $repoRoot '.tools\terraform'
$terraform = Join-Path $toolsDirectory 'terraform.exe'
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

function Invoke-Terraform {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & $terraform "-chdir=$terraformDirectory" @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Terraform failed: $($Arguments -join ' ')" }
}

function Invoke-Aws {
    param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
    & aws @Arguments --profile $Profile --region $Region
    if ($LASTEXITCODE -ne 0) { throw "AWS CLI failed: aws $($Arguments -join ' ')" }
}

function Get-TerraformOutput {
    param([string]$Name)
    $value = & $terraform "-chdir=$terraformDirectory" output -raw $Name
    if ($LASTEXITCODE -ne 0) { throw "Missing Terraform output: $Name" }
    return $value.Trim()
}

function New-ReviewedPlan {
    $accountId = Invoke-Aws sts get-caller-identity --query Account --output text
    $stateBucket = "ras-backend-tfstate-$accountId-$Region"
    $null = Invoke-Terraform init -input=false "-backend-config=bucket=$stateBucket" "-backend-config=region=$Region" "-backend-config=profile=$Profile"
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

if ($Action -eq 'up') {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Docker is required. Start Docker Desktop, then rerun this command.' }
    docker info --format '{{.ServerVersion}}' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Docker Engine is not running.' }
    $dirty = @(git -C $repoRoot status --porcelain -- services database deploy/aws-dev-deploy.sh infra/terraform)
    if ($LASTEXITCODE -ne 0) { throw 'Could not inspect Git status.' }
    if ($dirty.Count -gt 0) { throw 'Commit or merge backend, migration, deployment, and Terraform changes before using this repeatable deploy command.' }
    $imageTag = (git -C $repoRoot rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $imageTag -notmatch '^[a-f0-9]{40}$') { throw 'Could not determine the source commit SHA.' }

    $changes = New-ReviewedPlan
    if ($changes.Count -gt 0) {
        Write-Output 'Applying the reviewed non-destructive development plan.'
        Invoke-Terraform apply -input=false -auto-approve $planPath
    } else { Write-Output 'Infrastructure is already current.' }

    $account = Invoke-Aws sts get-caller-identity --query Account --output text
    $registry = "$account.dkr.ecr.$Region.amazonaws.com"
    $login = Invoke-Aws ecr get-login-password
    $login | docker login --username AWS --password-stdin $registry | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Docker could not authenticate to ECR.' }

    $images = @(
        @{ Name = 'ras-auth-api'; Context = 'services\auth-api'; Dockerfile = 'services\auth-api\Dockerfile' },
        @{ Name = 'ras-branch-api'; Context = 'services\branch-api'; Dockerfile = 'services\branch-api\Dockerfile' },
        @{ Name = 'ras-attendance-api'; Context = 'services\attendance-api'; Dockerfile = 'services\attendance-api\Dockerfile' },
        @{ Name = 'ras-db-migrations'; Context = '.'; Dockerfile = 'database\Dockerfile' }
    )
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

    $instanceId = Get-TerraformOutput 'application_instance_id'
    $gatewayUrl = Get-TerraformOutput 'api_gateway_base_url'
    $deadline = [DateTime]::UtcNow.AddMinutes(15)
    do {
        Start-Sleep -Seconds 10
        $ping = Invoke-Aws ssm describe-instance-information --filters "Key=InstanceIds,Values=$instanceId" --query 'InstanceInformationList[0].PingStatus' --output text
    } while ($ping -ne 'Online' -and [DateTime]::UtcNow -lt $deadline)
    if ($ping -ne 'Online') { throw "EC2 host $instanceId did not become SSM-online." }
    Invoke-Ssm $instanceId 'sudo /usr/local/bin/ras-host-doctor.sh'
    Invoke-Ssm $instanceId "sudo /usr/local/bin/ras-backend-deploy.sh migrate $imageTag"
    foreach ($service in @('auth-api', 'branch-api', 'attendance-api')) {
        Invoke-Ssm $instanceId "sudo /usr/local/bin/ras-backend-deploy.sh $service $imageTag"
    }
    try {
        $health = Invoke-WebRequest -Uri "$gatewayUrl/health" -TimeoutSec 25 -UseBasicParsing
        if ([int]$health.StatusCode -ne 200) { throw "Gateway health returned HTTP $([int]$health.StatusCode)." }
    } catch { throw "Development deployment finished, but API Gateway health verification failed: $($_.Exception.Message)" }
    Write-Output "Development APIs are available at $gatewayUrl"
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
    Invoke-Terraform init -input=false
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
    $confirmation = Read-Host 'Type DELETE RAS BACKEND DEV DATA to continue'
    if ($confirmation -cne 'DELETE RAS BACKEND DEV DATA') { throw 'Teardown cancelled; no resources were destroyed.' }
    Invoke-Terraform apply -input=false -auto-approve $planPath
    Write-Output 'Development infrastructure destroyed. Terraform state bucket and encrypted laptop backup remain.'
}
