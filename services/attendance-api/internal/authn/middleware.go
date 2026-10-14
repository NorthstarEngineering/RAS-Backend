package authn

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

const (
    UserIDKey = "user_id"
    RoleKey = "user_role"
    BranchIDKey = "branch_id"
    ClaimsKey = "authn.claims"
    Issuer = "restaurant-backend"
)

type Claims struct {
    UserID uuid.UUID
    Role string
    BranchID string
}

type tokenClaims struct {
    Subject string `json:"sub"`
    Role string `json:"role"`
    BranchID string `json:"branch_id"`
    MustChangePassword bool `json:"mcp"`
    ExpiresAt int64 `json:"exp"`
    NotBefore int64 `json:"nbf"`
    Issuer string `json:"iss"`
}

type tokenHeader struct { Algorithm string `json:"alg"`; Type string `json:"typ"` }

func NormalizeRole(role string) (string, bool) {
    switch strings.ToUpper(strings.TrimSpace(role)) {
    case "CEO": return "CEO", true
    case "BM", "BRANCH_MANAGER": return "BRANCH_MANAGER", true
    case "SUPPLY_MANAGER": return "SUPPLY_MANAGER", true
    default: return "", false
    }
}

func Verify(raw string, secret []byte, now time.Time) (Claims, error) {
    parts := strings.Split(raw, ".")
    if len(parts) != 3 || len(secret) < 32 { return Claims{}, errors.New("invalid token") }
    headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0]); if err != nil { return Claims{}, errors.New("invalid token") }
    var header tokenHeader
    if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" { return Claims{}, errors.New("invalid token") }
    mac := hmac.New(sha256.New, secret); _, _ = mac.Write([]byte(parts[0]+"."+parts[1]))
    signature, err := base64.RawURLEncoding.DecodeString(parts[2]); if err != nil || !hmac.Equal(signature, mac.Sum(nil)) { return Claims{}, errors.New("invalid token") }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1]); if err != nil { return Claims{}, errors.New("invalid token") }
    var claims tokenClaims
    if json.Unmarshal(payload, &claims) != nil || claims.Issuer != Issuer || claims.ExpiresAt == 0 || now.Unix() >= claims.ExpiresAt || (claims.NotBefore != 0 && now.Unix() < claims.NotBefore) || claims.MustChangePassword {
        return Claims{}, errors.New("invalid token")
    }
    userID, err := uuid.Parse(claims.Subject); if err != nil { return Claims{}, errors.New("invalid token") }
    role, ok := NormalizeRole(claims.Role); if !ok { return Claims{}, errors.New("invalid token") }
    if role == "BRANCH_MANAGER" {
        if _, err := uuid.Parse(claims.BranchID); err != nil { return Claims{}, errors.New("invalid token") }
    } else if claims.BranchID != "" { return Claims{}, errors.New("invalid token") }
    return Claims{UserID:userID, Role:role, BranchID:claims.BranchID}, nil
}

func Middleware(secret []byte) gin.HandlerFunc {
    return func(c *gin.Context) {
        scheme, raw, ok := strings.Cut(c.GetHeader("Authorization"), " ")
        if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(raw) == "" {
            c.AbortWithStatusJSON(401, gin.H{"error":gin.H{"code":"UNAUTHORIZED","message":"authentication required"}}); return
        }
        claims, err := Verify(strings.TrimSpace(raw), secret, time.Now())
        if err != nil { c.AbortWithStatusJSON(401, gin.H{"error":gin.H{"code":"UNAUTHORIZED","message":"invalid or expired token"}}); return }
        c.Set(ClaimsKey, claims); c.Set(UserIDKey, claims.UserID.String()); c.Set(RoleKey, claims.Role); c.Set(BranchIDKey, claims.BranchID)
        c.Next()
    }
}

func RequireRoles(roles ...string) gin.HandlerFunc {
    allowed := make(map[string]struct{}, len(roles))
    for _, role := range roles { if canonical, ok := NormalizeRole(role); ok { allowed[canonical] = struct{}{} } }
    return func(c *gin.Context) {
        role := c.GetString(RoleKey)
        if _, ok := allowed[role]; !ok { c.AbortWithStatusJSON(403, gin.H{"error":gin.H{"code":"FORBIDDEN","message":"operation is not permitted"}}); return }
        c.Next()
    }
}

func BranchScope(c *gin.Context, requested string) bool {
    if c.GetString(RoleKey) == "CEO" { return true }
    return c.GetString(RoleKey) == "BRANCH_MANAGER" && requested != "" && requested == c.GetString(BranchIDKey)
}
