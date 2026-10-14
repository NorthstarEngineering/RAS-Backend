package authn

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

func signedToken(t *testing.T, secret []byte, role, branch string, exp time.Time, mcp bool) string {
    t.Helper()
    header,_:=json.Marshal(tokenHeader{Algorithm:"HS256",Type:"JWT"})
    claims,_:=json.Marshal(tokenClaims{Subject:uuid.NewString(),Role:role,BranchID:branch,ExpiresAt:exp.Unix(),Issuer:Issuer,MustChangePassword:mcp})
    unsigned:=base64.RawURLEncoding.EncodeToString(header)+"."+base64.RawURLEncoding.EncodeToString(claims)
    mac:=hmac.New(sha256.New,secret);_,_=mac.Write([]byte(unsigned))
    return unsigned+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifyRequiresValidSignedUnexpiredJWT(t *testing.T) {
    secret:=[]byte(strings.Repeat("s",32));now:=time.Now()
    branchID:=uuid.NewString()
    valid:=signedToken(t,secret,"BRANCH_MANAGER",branchID,now.Add(time.Minute),false)
    claims,err:=Verify(valid,secret,now);if err!=nil{t.Fatalf("valid token rejected: %v",err)}
    if claims.Role!="BRANCH_MANAGER"||claims.BranchID!=branchID{t.Fatalf("unexpected claims: %+v",claims)}
    if _,ok:=NormalizeRole(" BM ");!ok{t.Fatal("legacy BM alias should normalize")}
    if _,err:=Verify(signedToken(t,secret,"CEO","",now,false),secret,now);err==nil{t.Fatal("expired token accepted")}
    if _,err:=Verify(signedToken(t,secret,"unknown","",now.Add(time.Minute),false),secret,now);err==nil{t.Fatal("unknown role accepted")}
    if _,err:=Verify(signedToken(t,secret,"CEO","",now.Add(time.Minute),true),secret,now);err==nil{t.Fatal("must-change-password token accepted")}
    if _,err:=Verify(valid,[]byte(strings.Repeat("x",32)),now);err==nil{t.Fatal("wrong signing key accepted")}
}

func TestMiddlewareDoesNotTrustIdentityHeaders(t *testing.T) {
    gin.SetMode(gin.TestMode);router:=gin.New();router.Use(Middleware([]byte(strings.Repeat("s",32))))
    router.GET("/protected",func(c *gin.Context){c.Status(204)})
    req:=httptest.NewRequest("GET","/protected",nil);req.Header.Set("X-User-ID",uuid.NewString());req.Header.Set("X-User-Role","CEO")
    res:=httptest.NewRecorder();router.ServeHTTP(res,req)
    if res.Code!=401{t.Fatalf("forged identity headers got status %d",res.Code)}
}

func TestRoleAndBranchAuthorization(t *testing.T) {
    gin.SetMode(gin.TestMode);router:=gin.New();router.Use(func(c *gin.Context){c.Set(RoleKey,"BRANCH_MANAGER");c.Set(BranchIDKey,"branch-a");c.Next()})
    router.POST("/ceo",RequireRoles("CEO"),func(c *gin.Context){c.Status(204)})
    router.GET("/branch/:id",func(c *gin.Context){if !BranchScope(c,c.Param("id")){c.AbortWithStatus(403);return};c.Status(204)})
    res:=httptest.NewRecorder();router.ServeHTTP(res,httptest.NewRequest("POST","/ceo",nil));if res.Code!=403{t.Fatalf("BM on CEO route status=%d",res.Code)}
    res=httptest.NewRecorder();router.ServeHTTP(res,httptest.NewRequest("GET","/branch/other",nil));if res.Code!=403{t.Fatalf("cross-branch request status=%d",res.Code)}
    res=httptest.NewRecorder();router.ServeHTTP(res,httptest.NewRequest("GET","/branch/branch-a",nil));if res.Code!=204{t.Fatalf("own-branch request status=%d",res.Code)}
}
