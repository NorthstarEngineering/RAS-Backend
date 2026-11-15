# RAS Backend Test Execution Report

## 1. Scope and execution policy

This report documents every test function added during this session. The system under test is the RAS Backend API, not the nopCommerce website. Execution evidence therefore comes from GitHub Actions rather than browser screenshots.

- **Execution environment:** GitHub Actions `ubuntu-latest`
- **Execution command:** `go test ./... -covermode=atomic -coverprofile=coverage.out -json`
- **Additional validation:** `go vet ./...`
- **Browser evidence:** N/A - backend API repository; no nopCommerce browser execution was requested or performed
- **Tester:** Copilot AI assistant
- **Execution date:** 2026-10-25

## 2. Execution summary

| Result | Count |
|---|---:|
| PASS | 32 |
| FAIL | 0 |
| BLOCKED | 4 |
| Total test functions | 36 |

The six feature-branch workflows completed successfully:

- [Central Kitchen Supply workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150040029)
- [Transfer API workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150042973)
- [Sales API workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150045948)
- [Menu API workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150261623)
- [Inventory API workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149687464)
- [Cost API workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150134638)

Each successful workflow ran formatting for the added test file, `go vet ./...`, unit tests, coverage collection, JSON test output, and artifact upload.

## 2.1 Failure investigation and remediation

The earlier failed workflow runs were not discarded. They were intermediate validation results caused by defects in the newly added test commits:

| Affected service | Failed run(s) | Failure cause | Resolution | Final run |
|---|---|---|---|---|
| Central Kitchen Supply | [40149664288](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149664288) | Malformed Go test-function nesting caused syntax errors before test execution | Rebuilt the test file with top-level test functions and pushed commit `4a8234c` | [40150040029](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150040029) - PASS |
| Transfer API | [40149669790](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149669790) | Malformed Go test-function nesting caused syntax errors before test execution | Rebuilt the test file with top-level test functions and pushed commit `2899df8` | [40150042973](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150042973) - PASS |
| Sales API | [40149676113](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149676113) | Malformed Go test-function nesting caused syntax errors before test execution | Rebuilt the test file with top-level test functions and pushed commit `b21d83f` | [40150045948](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150045948) - PASS |
| Menu API | [40149681460](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149681460), [40150120981](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150120981), [40150200005](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150200005) | First syntax errors, then Go vet rejected pointer-receiver calls on non-addressable composite literals | Rebuilt the test file and changed receiver setup to addressable variables; final commit `cd685eb` | [40150261623](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150261623) - PASS |
| Cost API | [40149693336](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40149693336), [40150054114](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150054114) | First syntax errors, then Go vet rejected a pointer-receiver call on a non-addressable composite literal | Rebuilt the test file and changed receiver setup to addressable variables; final commit `62843d5` | [40150134638](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40150134638) - PASS |

These failures occurred before the affected test cases could execute; they were test-code/CI validation defects, not reported failures of the RAS Backend business logic. The final runs passed `gofmt`, `go vet ./...`, and `go test ./...`.

## 3. Test cases

### TC-CENTRAL-001

1. **Test Case ID:** TC-CENTRAL-001
2. **Parent User Story:** Central Kitchen goods and stock validation
3. **Test Case Title:** Validate inventory movement type recognition
4. **Requirement / Acceptance Criterion:** `IN`, `OUT`, and `ADJUSTMENT` movement types are accepted; unknown movement types are rejected.
5. **Preconditions:** Central Kitchen Supply branch is checked out; Go module dependencies are available in GitHub Actions.
6. **Exact Test Data:** `IN`, `OUT`, `ADJUSTMENT`, and `UNKNOWN`.
7. **Numbered Test Steps:**
   1. Construct each movement type.
   2. Call `MovementType.IsValid()`.
   3. Compare the result with the expected validity.
8. **Expected Result:** The three supported values return `true`; `UNKNOWN` returns `false`.
9. **Testing Technique:** Table-driven unit test; equivalence partitioning.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Supported movement types were accepted and the unknown value was rejected.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Central Kitchen workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148073855).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-TRANSFER-001

1. **Test Case ID:** TC-TRANSFER-001
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate transfer branch authorization
4. **Requirement / Acceptance Criterion:** CEO users can manage any branch; branch managers can manage only their own branch; unsupported roles are denied.
5. **Preconditions:** Transfer API branch is checked out; two distinct branch UUIDs are available.
6. **Exact Test Data:** CEO identity; Branch Manager identity for branch A; branch A and branch B; unsupported `SUPPLY_MANAGER` role.
7. **Numbered Test Steps:**
   1. Create identities for CEO and Branch Manager.
   2. Call `Identity.ManagesBranch()` for the permitted and non-permitted branches.
   3. Compare each result with the authorization requirement.
8. **Expected Result:** CEO access is allowed for both branches; the manager is allowed only for branch A; unsupported role access is denied.
9. **Testing Technique:** Table-driven unit test; role and boundary-value authorization testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All authorization outcomes matched the acceptance criteria.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Transfer workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148068099).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-TRANSFER-002

1. **Test Case ID:** TC-TRANSFER-002
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate quantity rounding for transfers
4. **Requirement / Acceptance Criterion:** Transfer quantities are rounded to two decimal places using financial rounding.
5. **Preconditions:** Transfer API branch is checked out.
6. **Exact Test Data:** `1.234 -> 1.23`; `1.235 -> 1.24`; `12.999 -> 13.00`.
7. **Numbered Test Steps:**
   1. Pass each input to `round2`.
   2. Compare the returned value with the expected two-decimal result.
8. **Expected Result:** Every value is rounded to the expected financial precision.
9. **Testing Technique:** Table-driven unit test; boundary-value analysis.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All rounding examples returned the expected values.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Transfer workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148068099).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-SALES-001

1. **Test Case ID:** TC-SALES-001
2. **Parent User Story:** Sales, discounts, and shift closing
3. **Test Case Title:** Validate sale amount relationships
4. **Requirement / Acceptance Criterion:** Gross sales amount equals discount amount plus net sales amount.
5. **Preconditions:** Sales API branch is checked out.
6. **Exact Test Data:** Gross amount `100`; discount amount `15`; net amount `85`.
7. **Numbered Test Steps:**
   1. Construct a sale using the exact amounts.
   2. Compare gross amount with discount plus net amount.
8. **Expected Result:** `100 = 15 + 85`.
9. **Testing Technique:** Unit test; invariant testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** The sale amount invariant held.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Sales workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148079355).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-SALES-002

1. **Test Case ID:** TC-SALES-002
2. **Parent User Story:** Sales, discounts, and shift closing
3. **Test Case Title:** Validate discount lifecycle states
4. **Requirement / Acceptance Criterion:** A newly created discount is active and a canceled discount is represented as canceled.
5. **Preconditions:** Sales API branch is checked out.
6. **Exact Test Data:** Active discount amount `20`; lifecycle states `ACTIVE` and `CANCELED`.
7. **Numbered Test Steps:**
   1. Construct an active discount with amount `20`.
   2. Verify the initial status and positive amount.
   3. Transition the status to `CANCELED`.
   4. Verify the canceled state.
8. **Expected Result:** The discount begins as active and ends as canceled.
9. **Testing Technique:** Unit test; state-transition testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Both lifecycle states were represented correctly.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Sales workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148079355).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-MENU-001

1. **Test Case ID:** TC-MENU-001
2. **Parent User Story:** Company-wide master menu
3. **Test Case Title:** Validate menu item response mapping
4. **Requirement / Acceptance Criterion:** Public response fields preserve menu item identity, SKU, English name, selling price, active state, and RFC3339 timestamps.
5. **Preconditions:** Menu API branch is checked out; Menu API startup import defect is fixed.
6. **Exact Test Data:** Item ID `item-1`; SKU `SKU-1`; Arabic name `برجر`; English name `Burger`; selling price `125`; UTC timestamp `2026-10-25T00:00:00Z`.
7. **Numbered Test Steps:**
   1. Construct a `MenuItem` using the exact data.
   2. Call `MenuItem.ToResponse()`.
   3. Compare the mapped public fields and timestamp.
8. **Expected Result:** All public fields and timestamp formatting match the source item.
9. **Testing Technique:** Unit test; data-mapping verification.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** The response mapping matched the source values.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Menu workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148183256).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** GitHub Actions initially detected a missing `os` import and malformed duplicate import in `cmd/api/main.go`. The issue was fixed in [commit `4cb0c4d`](https://github.com/NorthstarEngineering/RAS-Backend/commit/4cb0c4d), and the rerun passed.

### TC-INVENTORY-001

1. **Test Case ID:** TC-INVENTORY-001
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Validate inventory status, value, and shortage
4. **Requirement / Acceptance Criterion:** Inventory status is critical when quantity is below minimum stock; total value and shortage are calculated correctly.
5. **Preconditions:** Inventory API branch is checked out.
6. **Exact Test Data:** Quantity `3`; unit cost `10`; minimum stock `5`; reorder level `8`.
7. **Numbered Test Steps:**
   1. Construct an inventory item with the exact values.
   2. Call `Status()`, `TotalValue()`, and `Shortage()`.
   3. Compare results with `CRITICAL`, `30`, and `2`.
8. **Expected Result:** Status is `CRITICAL`; total value is `30`; shortage is `2`.
9. **Testing Technique:** Unit test; boundary and arithmetic verification.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All three calculations matched the expected values.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Inventory workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148090815).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-INVENTORY-002

1. **Test Case ID:** TC-INVENTORY-002
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Validate receipt and issue movement calculations
4. **Requirement / Acceptance Criterion:** Receipts increase stock and record the received quantity; issues decrease stock and record the issued quantity.
5. **Preconditions:** Inventory API branch is checked out.
6. **Exact Test Data:** Starting quantity `10`; receipt quantity `4`; issue quantity `3`.
7. **Numbered Test Steps:**
   1. Apply an `IN` movement of `4` to quantity `10`.
   2. Verify resulting quantity `14` and recorded quantity `4`.
   3. Apply an `OUT` movement of `3` to quantity `10`.
   4. Verify resulting quantity `7` and recorded quantity `3`.
8. **Expected Result:** Receipt produces `14/4`; issue produces `7/3`.
9. **Testing Technique:** Unit test; state-transition and arithmetic testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Receipt and issue calculations matched the expected balances and ledger quantities.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Inventory workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148090815).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-COST-001

1. **Test Case ID:** TC-COST-001
2. **Parent User Story:** Cost management
3. **Test Case Title:** Validate menu item costing calculations
4. **Requirement / Acceptance Criterion:** Gross profit, food-cost percentage, recipe component mapping, and last-updated timestamp are calculated and returned correctly.
5. **Preconditions:** Cost API branch is checked out.
6. **Exact Test Data:** Menu item `menu-1`; selling price `100`; cost amount `40`; component `good-1`, quantity `2`, unit `kg`; timestamp `2026-10-25T00:00:00Z`.
7. **Numbered Test Steps:**
   1. Construct a menu item cost with the exact data.
   2. Call `ToCostingResponse(100, components)`.
   3. Verify gross profit, food-cost percentage, component mapping, and timestamp.
8. **Expected Result:** Gross profit is `60`; food-cost percentage is `40%`; component data is preserved; timestamp is RFC3339.
9. **Testing Technique:** Unit test; arithmetic and response-mapping verification.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All costing calculations and mapped fields matched the expected values.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; see [Cost workflow](https://github.com/NorthstarEngineering/RAS-Backend/actions/runs/40148095741).
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`; Go module test execution.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** No defect found.

### TC-BRANCH-001

1. **Test Case ID:** TC-BRANCH-001
2. **Parent User Story:** Branch management
3. **Test Case Title:** Validate branch creation normalization and persistence
4. **Requirement / Acceptance Criterion:** Valid branch input is trimmed, time-normalized, marked active, timestamped, and passed to the repository.
5. **Preconditions:** Branch API test file exists in the local `feature/tests-and-ci` worktree; Go toolchain and dependencies are required.
6. **Exact Test Data:** Name `  Downtown  `; type `branch`; address `  Main Street  `; start time ` 9:05 `; late rate `125.50`; fixed UTC timestamp `2026-10-25T03:00:00Z`.
7. **Numbered Test Steps:**
   1. Create a service with a fake repository and fixed clock.
   2. Call `Create()` with the exact request.
   3. Verify normalization, active state, timestamps, and repository persistence.
8. **Expected Result:** Name becomes `Downtown`; address becomes `Main Street`; time becomes `09:05`; branch is active and persisted.
9. **Testing Technique:** Unit test; fake dependency and normalization testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Not executed in GitHub Actions. The local worktree contains unrelated uncommitted changes and the test file was not pushed to a branch workflow.
12. **Status:** BLOCKED
13. **Evidence (screenshot):** N/A; no GitHub Actions execution exists for this test case.
14. **Browser / Environment:** Local Windows checkout; GitHub Actions execution not available for this test case.
15. **Execution Date:** Not executed.
16. **Notes / Investigation:** Push the Branch API test file to a dedicated branch or merge it into the CI branch, then run the workflow before marking this case PASS or FAIL.

### TC-BRANCH-002

1. **Test Case ID:** TC-BRANCH-002
2. **Parent User Story:** Branch management
3. **Test Case Title:** Validate branch creation rejection rules
4. **Requirement / Acceptance Criterion:** Invalid names, types, times, and late rates are rejected with the correct domain errors.
5. **Preconditions:** Same as TC-BRANCH-001.
6. **Exact Test Data:** Empty name; 101-character name; type `warehouse`; time `9am`; missing rate; negative rate; rate `1.001`; rate `10000.01`.
7. **Numbered Test Steps:**
   1. Submit each invalid request to `Create()`.
   2. Compare the returned error with the expected domain error.
8. **Expected Result:** Each invalid input is rejected with its corresponding validation error.
9. **Testing Technique:** Table-driven unit test; negative testing and equivalence partitioning.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Not executed in GitHub Actions.
12. **Status:** BLOCKED
13. **Evidence (screenshot):** N/A; no GitHub Actions execution exists for this test case.
14. **Browser / Environment:** Local Windows checkout; GitHub Actions execution not available for this test case.
15. **Execution Date:** Not executed.
16. **Notes / Investigation:** Requires a pushed Branch API test workflow.

### TC-BRANCH-003

1. **Test Case ID:** TC-BRANCH-003
2. **Parent User Story:** Branch management
3. **Test Case Title:** Validate update no-op and changed-field persistence
4. **Requirement / Acceptance Criterion:** An empty update must not write or change `updated_at`; a changed update must normalize fields and persist exactly once.
5. **Preconditions:** Same as TC-BRANCH-001.
6. **Exact Test Data:** Existing branch ID; original name `Old Name`; new name ` New Name `; new address ` New Address `; time `10:30`; rate `20.00`; active state `false`.
7. **Numbered Test Steps:**
   1. Call `Update()` with an empty request and verify no repository update.
   2. Call `Update()` with changed fields.
   3. Verify normalized values, timestamp, and one repository update.
8. **Expected Result:** No-op update performs no write; changed update persists normalized values once.
9. **Testing Technique:** Unit test; state-change and interaction testing with a fake repository.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Not executed in GitHub Actions.
12. **Status:** BLOCKED
13. **Evidence (screenshot):** N/A; no GitHub Actions execution exists for this test case.
14. **Browser / Environment:** Local Windows checkout; GitHub Actions execution not available for this test case.
15. **Execution Date:** Not executed.
16. **Notes / Investigation:** Requires a pushed Branch API test workflow.

### TC-BRANCH-004

1. **Test Case ID:** TC-BRANCH-004
2. **Parent User Story:** Branch management
3. **Test Case Title:** Validate branch revenue rules
4. **Requirement / Acceptance Criterion:** Regular branches generate revenue; Central Kitchens do not.
5. **Preconditions:** Same as TC-BRANCH-001.
6. **Exact Test Data:** `TypeBranch`; `TypeCentralKitchen`.
7. **Numbered Test Steps:**
   1. Call `GeneratesRevenue()` for `TypeBranch`.
   2. Call `GeneratesRevenue()` for `TypeCentralKitchen`.
   3. Compare each result with the business rule.
8. **Expected Result:** Regular branch returns `true`; Central Kitchen returns `false`.
9. **Testing Technique:** Unit test; decision-table testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Not executed in GitHub Actions.
12. **Status:** BLOCKED
13. **Evidence (screenshot):** N/A; no GitHub Actions execution exists for this test case.
14. **Browser / Environment:** Local Windows checkout; GitHub Actions execution not available for this test case.
15. **Execution Date:** Not executed.
16. **Notes / Investigation:** Requires a pushed Branch API test workflow.

## 4. Evidence and limitations

- The six feature service workflows passed in GitHub Actions and uploaded coverage/test-result artifacts.
- The initial Menu workflow exposed and enabled correction of a real compile defect before the successful rerun.
- The four Branch API cases are documented but intentionally marked BLOCKED. They must not be reported as executed or passed until a GitHub Actions run exists.
- This report does not claim nopCommerce browser execution, screenshots, or Jira subtask updates.

## 5. Expanded boundary and negative test cases

The following cases were added after the initial execution report. Each entry uses the mandatory fields 1-16. The GitHub Actions links above are the execution evidence for the corresponding service.

### TC-CENTRAL-002 - `TestInventoryMovementTypesRequirePositiveQuantity`

1. **Test Case ID:** TC-CENTRAL-002
2. **Parent User Story:** Central Kitchen goods and stock validation
3. **Test Case Title:** Validate supported inventory movement categories
4. **Requirement / Acceptance Criterion:** Receipt, issue, and adjustment categories remain recognized.
5. **Preconditions:** Central Kitchen Go module available in CI.
6. **Exact Test Data:** `IN`, `OUT`, `ADJUSTMENT`.
7. **Numbered Test Steps:** Construct each movement type; call `IsValid`; compare with `true`.
8. **Expected Result:** Every supported category is valid.
9. **Testing Technique:** Equivalence partitioning.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All supported categories were valid.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-CENTRAL-003 - `TestInventoryModelPreservesMovementMetadata`

1. **Test Case ID:** TC-CENTRAL-003
2. **Parent User Story:** Central Kitchen goods and stock validation
3. **Test Case Title:** Validate movement audit metadata.
4. **Requirement / Acceptance Criterion:** Movement records preserve identity, inventory reference, quantity, reason, and actor.
5. **Preconditions:** Central Kitchen Go module available in CI.
6. **Exact Test Data:** Movement `movement-1`, inventory `inventory-1`, quantity `12.5`, reason `supplier receipt`, actor `user-1`.
7. **Numbered Test Steps:** Construct movement; inspect all audit fields.
8. **Expected Result:** No required audit field is empty or invalid.
9. **Testing Technique:** Data-integrity unit test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Required metadata was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-CENTRAL-004 - `TestSupplyRequestStartsWithPendingStatus`

1. **Test Case ID:** TC-CENTRAL-004
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate new supply request state.
4. **Requirement / Acceptance Criterion:** New supply requests start in `PENDING`.
5. **Preconditions:** Central Kitchen Go module available in CI.
6. **Exact Test Data:** Request branch `branch-1`, priority `NORMAL`, status `PENDING`.
7. **Numbered Test Steps:** Construct request; inspect status.
8. **Expected Result:** Status equals `PENDING`.
9. **Testing Technique:** State-transition boundary test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Request status was `PENDING`.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-CENTRAL-005 - `TestSupplyRequestItemPreservesRequestedQuantity`

1. **Test Case ID:** TC-CENTRAL-005
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate fractional request quantities.
4. **Requirement / Acceptance Criterion:** Small positive quantities are preserved without truncation.
5. **Preconditions:** Central Kitchen Go module available in CI.
6. **Exact Test Data:** Requested quantity `0.001`.
7. **Numbered Test Steps:** Construct a supply request item; inspect `RequestedQty`.
8. **Expected Result:** Quantity remains exactly `0.001`.
9. **Testing Technique:** Boundary-value and precision testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Fractional quantity was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-TRANSFER-003 - `TestTransferStatusesAreValidOnlyForKnownLifecycleStates`

1. **Test Case ID:** TC-TRANSFER-003
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate transfer lifecycle status boundaries.
4. **Requirement / Acceptance Criterion:** Pending, approved, rejected, sent, and received are valid; unknown status is invalid.
5. **Preconditions:** Transfer Go module available in CI.
6. **Exact Test Data:** Five supported statuses and `CANCELLED`.
7. **Numbered Test Steps:** Validate every supported status; validate unknown status.
8. **Expected Result:** Supported statuses pass; unknown status fails.
9. **Testing Technique:** Equivalence partitioning.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Status validation matched the lifecycle contract.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-TRANSFER-004 - `TestIdentityDoesNotManageZeroBranchForBranchManager`

1. **Test Case ID:** TC-TRANSFER-004
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Reject branch-manager access to zero branch.
4. **Requirement / Acceptance Criterion:** A branch manager cannot manage an unassigned zero UUID branch.
5. **Preconditions:** Transfer Go module available in CI.
6. **Exact Test Data:** Manager branch UUID and destination `uuid.Nil`.
7. **Numbered Test Steps:** Call `ManagesBranch(uuid.Nil)`.
8. **Expected Result:** Result is false.
9. **Testing Technique:** Negative authorization and boundary testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Zero-branch access was denied.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-TRANSFER-005 - `TestTransferPrioritiesHaveStableWireValues`

1. **Test Case ID:** TC-TRANSFER-005
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate transfer priority wire values.
4. **Requirement / Acceptance Criterion:** Normal and urgent priorities use stable API values.
5. **Preconditions:** Transfer Go module available in CI.
6. **Exact Test Data:** `NORMAL`, `URGENT`.
7. **Numbered Test Steps:** Compare constants with their wire values.
8. **Expected Result:** Both values remain stable.
9. **Testing Technique:** Contract/value test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Priority values matched the contract.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-TRANSFER-006 - `TestTransferActionsHaveAuditValues`

1. **Test Case ID:** TC-TRANSFER-006
2. **Parent User Story:** Supply requests and receipt
3. **Test Case Title:** Validate non-empty transfer audit actions.
4. **Requirement / Acceptance Criterion:** Every lifecycle action has a non-empty audit value.
5. **Preconditions:** Transfer Go module available in CI.
6. **Exact Test Data:** Requested, approved, rejected, sent, received actions.
7. **Numbered Test Steps:** Iterate through actions; verify each is non-empty.
8. **Expected Result:** No audit action is empty.
9. **Testing Technique:** Completeness and contract testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All audit actions were populated.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-SALES-003 - `TestSaleItemCalculatesLineTotalAtBoundaryQuantities`

1. **Test Case ID:** TC-SALES-003
2. **Parent User Story:** Sales, discounts, and shift closing
3. **Test Case Title:** Validate sale line totals at quantity boundaries.
4. **Requirement / Acceptance Criterion:** Single, fractional, and large quantities calculate correct line totals.
5. **Preconditions:** Sales Go module available in CI.
6. **Exact Test Data:** `1 x 12.5 = 12.5`; `0.5 x 10 = 5`; `100 x 2.25 = 225`.
7. **Numbered Test Steps:** Construct each line item; compare calculated total.
8. **Expected Result:** Every line total matches quantity multiplied by unit price.
9. **Testing Technique:** Table-driven boundary and arithmetic testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** All line totals were correct.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-SALES-004 - `TestShiftStartsWithOpeningFloatAndStatus`

1. **Test Case ID:** TC-SALES-004
2. **Parent User Story:** Sales, discounts, and shift closing
3. **Test Case Title:** Validate shift opening boundary with zero float.
4. **Requirement / Acceptance Criterion:** A shift may represent an opening float of zero and must be open at start.
5. **Preconditions:** Sales Go module available in CI.
6. **Exact Test Data:** Opening float `0`, status `OPEN`.
7. **Numbered Test Steps:** Construct shift; inspect opening values.
8. **Expected Result:** Zero is preserved and status is `OPEN`.
9. **Testing Technique:** Boundary-value state testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Opening state was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-SALES-005 - `TestCanceledDiscountKeepsCancellationReason`

1. **Test Case ID:** TC-SALES-005
2. **Parent User Story:** Sales, discounts, and shift closing
3. **Test Case Title:** Validate discount cancellation audit reason.
4. **Requirement / Acceptance Criterion:** A canceled discount retains its cancellation reason.
5. **Preconditions:** Sales Go module available in CI.
6. **Exact Test Data:** Status `CANCELED`; reason `manager correction`.
7. **Numbered Test Steps:** Construct canceled discount; inspect reason.
8. **Expected Result:** The reason is present and unchanged.
9. **Testing Technique:** Audit-field integrity testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Cancellation reason was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-MENU-002 - `TestMenuItemToResponseKeepsOptionalFieldsNil`

1. **Test Case ID:** TC-MENU-002
2. **Parent User Story:** Company-wide master menu
3. **Test Case Title:** Validate nil optional menu fields.
4. **Requirement / Acceptance Criterion:** Missing optional menu data remains absent rather than becoming fabricated values.
5. **Preconditions:** Menu Go module available in CI.
6. **Exact Test Data:** Item with ID, SKU, name, and zero selling price; all optional fields nil.
7. **Numbered Test Steps:** Map item to response; inspect optional pointers.
8. **Expected Result:** All optional pointers remain nil.
9. **Testing Technique:** Null/absence boundary testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Optional fields remained nil.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-MENU-003 - `TestMenuRequestToResponsePreservesApprovalState`

1. **Test Case ID:** TC-MENU-003
2. **Parent User Story:** Company-wide master menu
3. **Test Case Title:** Validate rejected menu request mapping.
4. **Requirement / Acceptance Criterion:** Rejection status, reason, and fractional requested price are preserved.
5. **Preconditions:** Menu Go module available in CI.
6. **Exact Test Data:** Status `REJECTED`; reason `duplicate item`; price `0.01`.
7. **Numbered Test Steps:** Map request; compare status, reason, and price.
8. **Expected Result:** All values match the source request.
9. **Testing Technique:** State and precision mapping test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Rejected request data was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-MENU-004 - `TestCategoryResponsePreservesSortOrderBoundary`

1. **Test Case ID:** TC-MENU-004
2. **Parent User Story:** Company-wide master menu
3. **Test Case Title:** Validate zero category sort order.
4. **Requirement / Acceptance Criterion:** Zero is a valid and preserved first sort order.
5. **Preconditions:** Menu Go module available in CI.
6. **Exact Test Data:** Sort order `0`; active category.
7. **Numbered Test Steps:** Map category; inspect sort order and active state.
8. **Expected Result:** Sort order remains zero and active state remains true.
9. **Testing Technique:** Boundary-value mapping test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Category boundary values were preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-MENU-005 - `TestMenuItemToResponseFormatsUTC`

1. **Test Case ID:** TC-MENU-005
2. **Parent User Story:** Company-wide master menu
3. **Test Case Title:** Validate UTC timestamp normalization.
4. **Requirement / Acceptance Criterion:** API timestamps are emitted as UTC RFC3339 values.
5. **Preconditions:** Menu Go module available in CI.
6. **Exact Test Data:** `2026-10-25 03:00 +03:00`.
7. **Numbered Test Steps:** Map item; inspect created and updated timestamps.
8. **Expected Result:** Both values equal `2026-10-25T00:00:00Z`.
9. **Testing Technique:** Time-zone boundary testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Timestamps were normalized to UTC.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-INVENTORY-003 - `TestApplyMovementRejectsInsufficientStock`

1. **Test Case ID:** TC-INVENTORY-003
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Reject stock issue above available balance.
4. **Requirement / Acceptance Criterion:** An OUT movement cannot reduce stock below zero.
5. **Preconditions:** Inventory Go module available in CI.
6. **Exact Test Data:** Current quantity `2`; issue quantity `3`.
7. **Numbered Test Steps:** Apply OUT movement; inspect returned error.
8. **Expected Result:** An insufficient-stock error is returned.
9. **Testing Technique:** Negative and boundary-value testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Insufficient stock was rejected.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-INVENTORY-004 - `TestApplyMovementSupportsAdjustmentDeltaAtZeroBoundary`

1. **Test Case ID:** TC-INVENTORY-004
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Allow adjustment to exact zero balance.
4. **Requirement / Acceptance Criterion:** A negative adjustment equal to current stock produces zero without going negative.
5. **Preconditions:** Inventory Go module available in CI.
6. **Exact Test Data:** Current quantity `2`; adjustment delta `-2`.
7. **Numbered Test Steps:** Apply adjustment; inspect new quantity and recorded delta.
8. **Expected Result:** New quantity is zero and recorded delta is `-2`.
9. **Testing Technique:** Boundary-value arithmetic testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Exact-zero adjustment succeeded.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-INVENTORY-005 - `TestNormalizePagingUsesDefaultsAndMaximum`

1. **Test Case ID:** TC-INVENTORY-005
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Validate paging lower and upper bounds.
4. **Requirement / Acceptance Criterion:** Invalid lower values use defaults and excessive limits are capped.
5. **Preconditions:** Inventory Go module available in CI.
6. **Exact Test Data:** Page `0`, limit `0`; page `-1`, limit `MaxLimit+1`.
7. **Numbered Test Steps:** Normalize both input pairs; compare results with defaults and maximum.
8. **Expected Result:** Defaults are applied and limit is capped at `MaxLimit`.
9. **Testing Technique:** Boundary-value testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Paging bounds were normalized correctly.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-INVENTORY-006 - `TestRoleAuthorizationRules`

1. **Test Case ID:** TC-INVENTORY-006
2. **Parent User Story:** Inventory management
3. **Test Case Title:** Validate inventory role permissions.
4. **Requirement / Acceptance Criterion:** CEO and Branch Manager may adjust; only CEO has all-branch access.
5. **Preconditions:** Inventory Go module available in CI.
6. **Exact Test Data:** Roles `CEO` and `BRANCH_MANAGER`.
7. **Numbered Test Steps:** Evaluate `CanAccessAllBranches` and `CanAdjust` for both roles.
8. **Expected Result:** CEO has both permissions; Branch Manager can adjust but cannot access all branches.
9. **Testing Technique:** Decision-table authorization testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Role permissions matched the contract.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-COST-002 - `TestCostingResponseAvoidsDivisionByZero`

1. **Test Case ID:** TC-COST-002
2. **Parent User Story:** Cost management
3. **Test Case Title:** Handle zero selling price safely.
4. **Requirement / Acceptance Criterion:** Food-cost percentage must not divide by zero.
5. **Preconditions:** Cost Go module available in CI.
6. **Exact Test Data:** Cost `10`; selling price `0`.
7. **Numbered Test Steps:** Build costing response; inspect food-cost percentage and gross profit.
8. **Expected Result:** Food-cost percentage is zero and gross profit is `-10`.
9. **Testing Technique:** Boundary and defensive arithmetic testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Zero-price calculation completed safely.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-COST-003 - `TestCostingResponseSupportsZeroCost`

1. **Test Case ID:** TC-COST-003
2. **Parent User Story:** Cost management
3. **Test Case Title:** Handle zero recipe cost.
4. **Requirement / Acceptance Criterion:** Zero cost produces full gross profit and zero food-cost percentage.
5. **Preconditions:** Cost Go module available in CI.
6. **Exact Test Data:** Cost `0`; selling price `25`.
7. **Numbered Test Steps:** Build costing response; inspect profit values.
8. **Expected Result:** Gross profit is `25`; food-cost percentage is zero.
9. **Testing Technique:** Boundary arithmetic testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Zero-cost calculation matched expectations.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-COST-004 - `TestCostingResponsePreservesComponentOrder`

1. **Test Case ID:** TC-COST-004
2. **Parent User Story:** Cost management
3. **Test Case Title:** Preserve recipe component order.
4. **Requirement / Acceptance Criterion:** Response component order matches recipe input order.
5. **Preconditions:** Cost Go module available in CI.
6. **Exact Test Data:** `good-1` quantity `1 kg`; `good-2` quantity `0.5 l`.
7. **Numbered Test Steps:** Build response with two components; compare positions.
8. **Expected Result:** `good-1` remains first and `good-2` remains second.
9. **Testing Technique:** Collection-order and mapping test.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Component order was preserved.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.

### TC-COST-005 - `TestCostingResponseUsesUTCForLastUpdated`

1. **Test Case ID:** TC-COST-005
2. **Parent User Story:** Cost management
3. **Test Case Title:** Validate costing timestamp normalization.
4. **Requirement / Acceptance Criterion:** Last-updated timestamp is returned in UTC RFC3339 format.
5. **Preconditions:** Cost Go module available in CI.
6. **Exact Test Data:** `2026-10-25 03:00 +03:00`.
7. **Numbered Test Steps:** Build response; inspect `LastUpdated`.
8. **Expected Result:** Timestamp equals `2026-10-25T00:00:00Z`.
9. **Testing Technique:** Time-zone boundary testing.
10. **Tester:** Copilot AI assistant.
11. **Actual Result:** Timestamp was normalized to UTC.
12. **Status:** PASS
13. **Evidence (screenshot):** N/A; GitHub Actions artifact and workflow.
14. **Browser / Environment:** GitHub Actions `ubuntu-latest`.
15. **Execution Date:** 2026-10-25.
16. **Notes / Investigation:** None.
