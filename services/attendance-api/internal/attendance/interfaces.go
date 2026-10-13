package attendance

import (
	"context"
	"errors"
)

// ============================================================
// Interfaces — العقود بين Attendance وباقي الموديولات
//
// القاعدة الذهبية:
//   - Attendance معرّف الـ interfaces اللي محتاجها بنفسه
//   - مش بنستورد كود من موديول Branch أو Employee مباشرة
//   - عند الدمج، صاحب موديول Branch هيعمل implement للـ BranchReader
//
// الفايدة:
//   - استقلالية كاملة في التطوير
//   - سهولة الاختبار (نستخدم mocks)
//   - الدمج بيتم في main.go بس
// ============================================================

// ============================================================
// EmployeeReader — اللي محتاجينه من موديول Employee
// Ref: FR-EMP-001..006
// ============================================================

// EmployeeInfo — معلومات مختصرة عن الموظف
// دي مش الـ Employee الكامل، دي بس اللي Attendance محتاجه
type EmployeeInfo struct {
	ID       string
	BranchID string
	Active   bool
}

type EmployeeReader interface {
	// GetByID — يجيب موظف بالـ ID
	// لو مش موجود → يرجع (nil, nil)
	// لو فيه مشكلة تقنية → يرجع (nil, error)
	GetByID(ctx context.Context, id string) (*EmployeeInfo, error)
}

// ============================================================
// BranchReader — اللي محتاجينه من موديول Branch
// Ref: FR-BRANCH-001..005
// ============================================================

// BranchInfo — معلومات مختصرة عن الفرع
type BranchInfo struct {
	ID string

	// ميعاد بداية الشغل الرسمي بصيغة "HH:MM"
	// Ref: FR-BRANCH-005, BR-006
	OfficialStartTime string

	// قيمة الخصم لكل ساعة تأخير مكتملة
	// Ref: FR-DED-002
	LatenessRatePerHour float64

	// هل ده Central Kitchen؟ (stock-only، مفيش حضور)
	// Ref: FR-BRANCH-003, BR-001
	IsCentralKitchen bool
}

type BranchReader interface {
	// GetByID — يجيب فرع بالـ ID
	// لو مش موجود → يرجع (nil, nil)
	// لو فيه مشكلة تقنية → يرجع (nil, error)
	GetByID(ctx context.Context, id string) (*BranchInfo, error)
}

// ============================================================
// Mock Implementations — للاختبار والتطوير المستقل
//
// نستخدمها دلوقتي لحد ما موديولات Employee و Branch تخلص
// ============================================================

// ---------- Mock Employee Reader ----------

type MockEmployeeReader struct {
	// map[employee_id]EmployeeInfo
	Employees map[string]EmployeeInfo
}

func NewMockEmployeeReader() *MockEmployeeReader {
	return &MockEmployeeReader{
		Employees: make(map[string]EmployeeInfo),
	}
}

// AddEmployee — لإضافة موظف للـ mock
func (m *MockEmployeeReader) AddEmployee(id, branchID string, active bool) {
	m.Employees[id] = EmployeeInfo{
		ID:       id,
		BranchID: branchID,
		Active:   active,
	}
}

func (m *MockEmployeeReader) GetByID(_ context.Context, id string) (*EmployeeInfo, error) {
	emp, ok := m.Employees[id]
	if !ok {
		return nil, nil // مش موجود (زي ما اتفقنا)
	}
	return &emp, nil
}

// ---------- Mock Branch Reader ----------

type MockBranchReader struct {
	// map[branch_id]BranchInfo
	Branches map[string]BranchInfo
}

func NewMockBranchReader() *MockBranchReader {
	return &MockBranchReader{
		Branches: make(map[string]BranchInfo),
	}
}

// AddBranch — لإضافة فرع للـ mock
func (m *MockBranchReader) AddBranch(id, startTime string, rate float64, isCK bool) {
	m.Branches[id] = BranchInfo{
		ID:                  id,
		OfficialStartTime:   startTime,
		LatenessRatePerHour: rate,
		IsCentralKitchen:    isCK,
	}
}

func (m *MockBranchReader) GetByID(_ context.Context, id string) (*BranchInfo, error) {
	br, ok := m.Branches[id]
	if !ok {
		return nil, nil
	}
	return &br, nil
}

// ============================================================
// Compile-time checks — نتأكد إن الـ mocks بتعمل implement صح
// ============================================================

var (
	_ EmployeeReader = (*MockEmployeeReader)(nil)
	_ BranchReader   = (*MockBranchReader)(nil)
)

// ============================================================
// حاجات مساعدة
// ============================================================

// ErrNotImplemented — للعمليات اللي لسه ما اتعملتش
var ErrNotImplemented = errors.New("not implemented")
