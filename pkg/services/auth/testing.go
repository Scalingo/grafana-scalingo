package auth

import (
	"context"
	"net"
	"time"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/user"
	"golang.org/x/oauth2"
)

type FakeUserAuthTokenService struct {
	CreateTokenProvider          func(ctx context.Context, user *user.User, clientIP net.IP, userAgent string) (*models.UserToken, error)
	TryRotateTokenProvider       func(ctx context.Context, token *models.UserToken, clientIP net.IP, userAgent string) (bool, error)
	LookupTokenProvider          func(ctx context.Context, unhashedToken string) (*models.UserToken, error)
	RevokeTokenProvider          func(ctx context.Context, token *models.UserToken, soft bool) error
	RevokeAllUserTokensProvider  func(ctx context.Context, userId int64) error
	ActiveAuthTokenCount         func(ctx context.Context) (int64, error)
	GetUserTokenProvider         func(ctx context.Context, userId, userTokenId int64) (*models.UserToken, error)
	GetUserTokensProvider        func(ctx context.Context, userId int64) ([]*models.UserToken, error)
	GetUserRevokedTokensProvider func(ctx context.Context, userId int64) ([]*models.UserToken, error)
	BatchRevokedTokenProvider    func(ctx context.Context, userIds []int64) error
}

func NewFakeUserAuthTokenService() *FakeUserAuthTokenService {
	return &FakeUserAuthTokenService{
		CreateTokenProvider: func(ctx context.Context, user *user.User, clientIP net.IP, userAgent string) (*models.UserToken, error) {
			return &models.UserToken{
				UserId:        0,
				UnhashedToken: "",
			}, nil
		},
		TryRotateTokenProvider: func(ctx context.Context, token *models.UserToken, clientIP net.IP, userAgent string) (bool, error) {
			return false, nil
		},
		LookupTokenProvider: func(ctx context.Context, unhashedToken string) (*models.UserToken, error) {
			return &models.UserToken{
				UserId:        0,
				UnhashedToken: "",
			}, nil
		},
		RevokeTokenProvider: func(ctx context.Context, token *models.UserToken, soft bool) error {
			return nil
		},
		RevokeAllUserTokensProvider: func(ctx context.Context, userId int64) error {
			return nil
		},
		BatchRevokedTokenProvider: func(ctx context.Context, userIds []int64) error {
			return nil
		},
		ActiveAuthTokenCount: func(ctx context.Context) (int64, error) {
			return 10, nil
		},
		GetUserTokenProvider: func(ctx context.Context, userId, userTokenId int64) (*models.UserToken, error) {
			return nil, nil
		},
		GetUserTokensProvider: func(ctx context.Context, userId int64) ([]*models.UserToken, error) {
			return nil, nil
		},
	}
}

// Init initializes the service.
// Required for dependency injection.
func (s *FakeUserAuthTokenService) Init() error {
	return nil
}

func (s *FakeUserAuthTokenService) CreateToken(ctx context.Context, user *user.User, clientIP net.IP, userAgent string) (*models.UserToken, error) {
	return s.CreateTokenProvider(context.Background(), user, clientIP, userAgent)
}

func (s *FakeUserAuthTokenService) LookupToken(ctx context.Context, unhashedToken string) (*models.UserToken, error) {
	return s.LookupTokenProvider(context.Background(), unhashedToken)
}

func (s *FakeUserAuthTokenService) TryRotateToken(ctx context.Context, token *models.UserToken, clientIP net.IP,
	userAgent string) (bool, error) {
	return s.TryRotateTokenProvider(context.Background(), token, clientIP, userAgent)
}

func (s *FakeUserAuthTokenService) RevokeToken(ctx context.Context, token *models.UserToken, soft bool) error {
	return s.RevokeTokenProvider(context.Background(), token, soft)
}

func (s *FakeUserAuthTokenService) RevokeAllUserTokens(ctx context.Context, userId int64) error {
	return s.RevokeAllUserTokensProvider(context.Background(), userId)
}

func (s *FakeUserAuthTokenService) ActiveTokenCount(ctx context.Context) (int64, error) {
	return s.ActiveAuthTokenCount(context.Background())
}

func (s *FakeUserAuthTokenService) GetUserToken(ctx context.Context, userId, userTokenId int64) (*models.UserToken, error) {
	return s.GetUserTokenProvider(context.Background(), userId, userTokenId)
}

func (s *FakeUserAuthTokenService) GetUserTokens(ctx context.Context, userId int64) ([]*models.UserToken, error) {
	return s.GetUserTokensProvider(context.Background(), userId)
}

func (s *FakeUserAuthTokenService) GetUserRevokedTokens(ctx context.Context, userId int64) ([]*models.UserToken, error) {
	return s.GetUserRevokedTokensProvider(context.Background(), userId)
}

func (s *FakeUserAuthTokenService) BatchRevokeAllUserTokens(ctx context.Context, userIds []int64) error {
	return s.BatchRevokedTokenProvider(ctx, userIds)
}

type FakeOAuthTokenService struct {
	passThruEnabled  bool
	ExpectedAuthUser *models.UserAuth
	ExpectedErrors   map[string]error
}

func (ts *FakeOAuthTokenService) GetCurrentOAuthToken(context.Context, *user.SignedInUser) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  ts.ExpectedAuthUser.OAuthAccessToken,
		RefreshToken: ts.ExpectedAuthUser.OAuthRefreshToken,
		Expiry:       ts.ExpectedAuthUser.OAuthExpiry,
		TokenType:    ts.ExpectedAuthUser.OAuthTokenType,
	}
}

func (ts *FakeOAuthTokenService) IsOAuthPassThruEnabled(*datasources.DataSource) bool {
	return ts.passThruEnabled
}

func (ts *FakeOAuthTokenService) HasOAuthEntry(context.Context, *user.SignedInUser) (*models.UserAuth, bool, error) {
	if ts.ExpectedAuthUser != nil {
		return ts.ExpectedAuthUser, true, nil
	}
	if error, ok := ts.ExpectedErrors["HasOAuthEntry"]; ok {
		return nil, false, error
	}
	return nil, false, nil
}

func (ts *FakeOAuthTokenService) InvalidateOAuthTokens(ctx context.Context, usr *models.UserAuth) error {
	ts.ExpectedAuthUser.OAuthAccessToken = ""
	ts.ExpectedAuthUser.OAuthRefreshToken = ""
	ts.ExpectedAuthUser.OAuthExpiry = time.Time{}
	return nil
}

func (ts *FakeOAuthTokenService) TryTokenRefresh(ctx context.Context, usr *models.UserAuth) error {
	if err, ok := ts.ExpectedErrors["TryTokenRefresh"]; ok {
		return err
	}
	return nil
}
