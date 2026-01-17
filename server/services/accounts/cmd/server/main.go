func main() {
	// 1. Config & Infra
	cfg := config.Load()
	db, _ := sql.Open("pgx", cfg.DB.DSN())

	// 2. Repository (Infra)
	userRepo := repository.NewPostgresUserRepo(db)

	// 3. Usecase (Business Logic) -> Depends on Repo Interface
	authUC := usecase.NewAuthUsecase(userRepo)

	// 4. Handler (Presentation) -> Depends on Usecase
	authHandler := handler.NewAuthHandler(authUC)

	// 5. Server Registration (Platform)
	srv := server.New(cfg.AppConfig)
	pb.RegisterAccountsServiceServer(srv.GrpcServer(), authHandler)

	srv.Run()
}