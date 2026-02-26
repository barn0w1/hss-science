package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	myaccountv1 "github.com/barn0w1/hss-science/server/bff/gen/myaccount/v1"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// Compile-time check that Server implements StrictServerInterface.
var _ myaccountv1.StrictServerInterface = (*Server)(nil)

// Server implements myaccountv1.StrictServerInterface for all JSON endpoints.
type Server struct {
	grpc         *grpcclient.Client
	sessionStore *session.Store
	devMode      bool
	logger       *slog.Logger
}

// NewServer creates a new strict server implementation.
func NewServer(
	grpc *grpcclient.Client,
	sessionStore *session.Store,
	devMode bool,
	logger *slog.Logger,
) *Server {
	return &Server{
		grpc:         grpc,
		sessionStore: sessionStore,
		devMode:      devMode,
		logger:       logger,
	}
}

func (s *Server) cookieName() string {
	if s.devMode {
		return "myaccount_session"
	}
	return session.CookieName
}

// --- Stub methods for manually-wired auth endpoints ---

// AuthLogin is handled by the manual chi handler, not the strict server.
func (s *Server) AuthLogin(_ context.Context, _ myaccountv1.AuthLoginRequestObject) (myaccountv1.AuthLoginResponseObject, error) {
	panic("unreachable: AuthLogin is wired as a manual chi handler")
}

// AuthCallback is handled by the manual chi handler, not the strict server.
func (s *Server) AuthCallback(_ context.Context, _ myaccountv1.AuthCallbackRequestObject) (myaccountv1.AuthCallbackResponseObject, error) {
	panic("unreachable: AuthCallback is wired as a manual chi handler")
}

// --- Auth JSON endpoints ---

// AuthLogout destroys the session and clears the cookie.
func (s *Server) AuthLogout(ctx context.Context, _ myaccountv1.AuthLogoutRequestObject) (myaccountv1.AuthLogoutResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	sessionID, _ := session.IDFromContext(ctx)

	// Delete session from Redis.
	if sessionID != "" {
		_ = s.sessionStore.Delete(ctx, sessionID)
	}

	// Clear the session cookie via the injected ResponseWriter.
	if w, ok := ResponseWriterFromContext(ctx); ok {
		http.SetCookie(w, &http.Cookie{
			Name:     s.cookieName(),
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   !s.devMode,
			SameSite: http.SameSiteLaxMode,
		})
	}

	if sd != nil {
		s.logger.Info("user logged out", "user_id", sd.UserID)
	}

	return myaccountv1.AuthLogout200JSONResponse(myaccountv1.MessageResponse{
		Message: "logged out",
	}), nil
}

// GetSession returns current session info for the SPA.
func (s *Server) GetSession(ctx context.Context, _ myaccountv1.GetSessionRequestObject) (myaccountv1.GetSessionResponseObject, error) {
	sd, ok := session.FromContext(ctx)
	if !ok {
		return myaccountv1.GetSession401JSONResponse{UnauthorizedJSONResponse: myaccountv1.UnauthorizedJSONResponse{
			Code: "UNAUTHENTICATED", Message: "no session",
		}}, nil
	}

	userID, err := uuid.Parse(sd.UserID)
	if err != nil {
		s.logger.Warn("invalid user_id in session data, forcing re-authentication", "user_id", sd.UserID, "error", err)
		return myaccountv1.GetSession401JSONResponse{UnauthorizedJSONResponse: myaccountv1.UnauthorizedJSONResponse{
			Code: "UNAUTHENTICATED", Message: "session is invalid, please log in again",
		}}, nil
	}

	var picture *string
	if sd.Picture != "" {
		picture = &sd.Picture
	}

	return myaccountv1.GetSession200JSONResponse(myaccountv1.SessionInfo{
		Authenticated: true,
		UserId:        userID,
		Email:         openapi_types.Email(sd.Email),
		GivenName:     sd.GivenName,
		FamilyName:    sd.FamilyName,
		Picture:       picture,
	}), nil
}

// --- Profile endpoints ---

// GetProfile returns the user's profile from the accounts service.
func (s *Server) GetProfile(ctx context.Context, _ myaccountv1.GetProfileRequestObject) (myaccountv1.GetProfileResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	resp, err := s.grpc.Service().GetProfile(grpcCtx, &pb.GetProfileRequest{})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return myaccountv1.GetProfile200JSONResponse(protoProfileToAPI(resp.Profile)), nil
}

// UpdateProfile updates the user's profile in the accounts service.
func (s *Server) UpdateProfile(ctx context.Context, request myaccountv1.UpdateProfileRequestObject) (myaccountv1.UpdateProfileResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	body := request.Body
	if body == nil {
		return myaccountv1.UpdateProfile400JSONResponse{BadRequestJSONResponse: myaccountv1.BadRequestJSONResponse{
			Code: "BAD_REQUEST", Message: "missing request body",
		}}, nil
	}

	// Build field mask and profile from non-nil fields.
	paths := make([]string, 0, 4)
	profile := &pb.Profile{}

	if body.GivenName != nil {
		profile.GivenName = *body.GivenName
		paths = append(paths, "given_name")
	}
	if body.FamilyName != nil {
		profile.FamilyName = *body.FamilyName
		paths = append(paths, "family_name")
	}
	if body.Picture != nil {
		profile.Picture = *body.Picture
		paths = append(paths, "picture")
	}
	if body.Locale != nil {
		profile.Locale = *body.Locale
		paths = append(paths, "locale")
	}

	if len(paths) == 0 {
		return myaccountv1.UpdateProfile400JSONResponse{BadRequestJSONResponse: myaccountv1.BadRequestJSONResponse{
			Code: "BAD_REQUEST", Message: "no fields to update",
		}}, nil
	}

	resp, err := s.grpc.Service().UpdateProfile(grpcCtx, &pb.UpdateProfileRequest{
		Profile:    profile,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return myaccountv1.UpdateProfile200JSONResponse(protoProfileToAPI(resp.Profile)), nil
}

// --- Linked accounts endpoints ---

// ListLinkedAccounts returns all linked identity providers for the user.
func (s *Server) ListLinkedAccounts(ctx context.Context, _ myaccountv1.ListLinkedAccountsRequestObject) (myaccountv1.ListLinkedAccountsResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	resp, err := s.grpc.Service().ListLinkedAccounts(grpcCtx, &pb.ListLinkedAccountsRequest{})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	accounts := make([]myaccountv1.LinkedAccount, len(resp.LinkedAccounts))
	for i, la := range resp.LinkedAccounts {
		accounts[i] = protoLinkedAccountToAPI(la)
	}

	return myaccountv1.ListLinkedAccounts200JSONResponse(myaccountv1.LinkedAccountList{
		LinkedAccounts: accounts,
	}), nil
}

// UnlinkAccount removes a linked identity provider.
func (s *Server) UnlinkAccount(ctx context.Context, request myaccountv1.UnlinkAccountRequestObject) (myaccountv1.UnlinkAccountResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	_, err := s.grpc.Service().UnlinkAccount(grpcCtx, &pb.UnlinkAccountRequest{
		LinkedAccountId: request.Id.String(),
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.FailedPrecondition || st.Code() == codes.AlreadyExists) {
			return myaccountv1.UnlinkAccount409JSONResponse(myaccountv1.Error{
				Code: "CONFLICT", Message: st.Message(),
			}), nil
		}
		return nil, mapGRPCError(err)
	}

	return myaccountv1.UnlinkAccount204Response{}, nil
}

// --- Sessions endpoints ---

// ListSessions returns all active sessions for the user.
func (s *Server) ListSessions(ctx context.Context, _ myaccountv1.ListSessionsRequestObject) (myaccountv1.ListSessionsResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	resp, err := s.grpc.Service().ListActiveSessions(grpcCtx, &pb.ListActiveSessionsRequest{})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	sessions := make([]myaccountv1.Session, len(resp.Sessions))
	for i, sess := range resp.Sessions {
		sessions[i] = protoSessionToAPI(sess)
	}

	return myaccountv1.ListSessions200JSONResponse(myaccountv1.SessionList{
		Sessions: sessions,
	}), nil
}

// RevokeSession revokes a specific session.
func (s *Server) RevokeSession(ctx context.Context, request myaccountv1.RevokeSessionRequestObject) (myaccountv1.RevokeSessionResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	_, err := s.grpc.Service().RevokeSession(grpcCtx, &pb.RevokeSessionRequest{
		SessionId: request.Id.String(),
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return myaccountv1.RevokeSession404JSONResponse{NotFoundJSONResponse: myaccountv1.NotFoundJSONResponse{
				Code: "NOT_FOUND", Message: st.Message(),
			}}, nil
		}
		return nil, mapGRPCError(err)
	}

	return myaccountv1.RevokeSession204Response{}, nil
}

// --- Account endpoints ---

// DeleteAccount permanently deletes the user's account.
func (s *Server) DeleteAccount(ctx context.Context, _ myaccountv1.DeleteAccountRequestObject) (myaccountv1.DeleteAccountResponseObject, error) {
	sd, _ := session.FromContext(ctx)
	grpcCtx := grpcclient.WithToken(ctx, sd.AccessToken)

	_, err := s.grpc.Service().DeleteAccount(grpcCtx, &pb.DeleteAccountRequest{})
	if err != nil {
		return nil, mapGRPCError(err)
	}

	return myaccountv1.DeleteAccount204Response{}, nil
}
