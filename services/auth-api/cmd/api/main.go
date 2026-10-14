package main

import (
    "context"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "strings"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/joho/godotenv"

    "restaurant-backend/internal/auth"
    "restaurant-backend/internal/database"
)

func main() {
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout,nil)))
    if err:=run();err!=nil{slog.Error("service stopped","service","auth-api","error",err);os.Exit(1)}
}

func run() error {
    _=godotenv.Load()
    authCfg,err:=auth.LoadConfig();if err!=nil{return err}
    startupCtx,startupCancel:=context.WithTimeout(context.Background(),10*time.Second);defer startupCancel()
    db,err:=database.Connect(startupCtx);if err!=nil{return err};defer db.Close()
    authModule:=auth.NewModule(auth.ModuleDeps{DB:db,Branches:branchChecker{db:db},Config:authCfg})
    if strings.EqualFold(os.Getenv("APP_ENV"),"development") {
        seedCtx,cancel:=context.WithTimeout(startupCtx,15*time.Second)
        err=authModule.Service.SeedCEO(seedCtx,os.Getenv("SEED_CEO_NAME"),os.Getenv("SEED_CEO_EMAIL"),os.Getenv("SEED_CEO_PASSWORD"));cancel()
        if err!=nil{return fmt.Errorf("development CEO bootstrap failed; verify explicit seed credentials")}
    }

    gin.SetMode(gin.ReleaseMode);router:=gin.New();router.Use(gin.Recovery(),requestLogging("auth-api"))
    if err:=router.SetTrustedProxies(nil);err!=nil{return fmt.Errorf("invalid trusted proxy configuration")}
    router.GET("/health",func(c *gin.Context){c.JSON(http.StatusOK,gin.H{"status":"ok","service":"auth-api"})})
    api:=router.Group("/api/v1");authModule.Handler.RegisterRoutes(api)
    port:=os.Getenv("PORT");if port==""{port="9001"}
    srv:=&http.Server{Addr:":"+port,Handler:router,ReadHeaderTimeout:5*time.Second,ReadTimeout:15*time.Second,WriteTimeout:30*time.Second,IdleTimeout:60*time.Second}
    serverErr:=make(chan error,1);go func(){serverErr<-srv.ListenAndServe()}();slog.Info("http server started","service","auth-api","port",port)
    ctx,stop:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer stop()
    select{case err:=<-serverErr:if err!=nil&&err!=http.ErrServerClosed{return fmt.Errorf("http server failed")};return nil
    case <-ctx.Done():shutdownCtx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();if err:=srv.Shutdown(shutdownCtx);err!=nil{return fmt.Errorf("http server shutdown failed")};return nil}
}

func requestLogging(service string)gin.HandlerFunc{
    return func(c *gin.Context){id:=strings.TrimSpace(c.GetHeader("X-Request-ID"));if id==""||len(id)>128{id=uuid.NewString()}else{for _,r:=range id{if !(r>='a'&&r<='z'||r>='A'&&r<='Z'||r>='0'&&r<='9'||r=='-'||r=='_'||r=='.'){id=uuid.NewString();break}}};c.Set("request_id",id);c.Header("X-Request-ID",id);start:=time.Now();c.Next();route:=c.FullPath();if route==""{route="unmatched"};slog.Info("http request","service",service,"request_id",id,"method",c.Request.Method,"route",route,"status",c.Writer.Status(),"duration_ms",time.Since(start).Milliseconds())}
}
