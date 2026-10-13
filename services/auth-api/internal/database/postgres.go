package database

import (
    "context"
    "database/sql"
    "fmt"
    "net"
    "net/url"
    "os"
    "strconv"
    "strings"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(ctx context.Context) (*sql.DB, error) {
    dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
    if dsn == "" {
        host, port, user, password, name := os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME")
        if host == "" || port == "" || user == "" || password == "" || name == "" { return nil, fmt.Errorf("set DATABASE_URL or all DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME values") }
        u := url.URL{Scheme:"postgres",User:url.UserPassword(user,password),Host:net.JoinHostPort(host,port),Path:name}
        q:=u.Query(); ssl:=os.Getenv("DB_SSLMODE");if ssl==""{ssl="disable"};q.Set("sslmode",ssl);u.RawQuery=q.Encode();dsn=u.String()
    }
    db, err := sql.Open("pgx",dsn); if err != nil { return nil,fmt.Errorf("invalid database configuration") }
	db.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS",5));db.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS",2));db.SetConnMaxLifetime(30*time.Minute);db.SetConnMaxIdleTime(5*time.Minute)
    pingCtx,cancel:=context.WithTimeout(ctx,8*time.Second);defer cancel()
    if err:=db.PingContext(pingCtx);err!=nil{_ = db.Close();return nil,fmt.Errorf("database is unavailable")}
    return db,nil
}

func envInt(key string,fallback int)int {value:=strings.TrimSpace(os.Getenv(key));if value==""{return fallback};n,err:=strconv.Atoi(value);if err!=nil||n<1{return fallback};return n}
