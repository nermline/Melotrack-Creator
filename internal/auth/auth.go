package auth

import (
	"fmt"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v3"
	"github.com/gin-gonic/gin"
	golangjwt "github.com/golang-jwt/jwt/v5"
	"github.com/nermline/Melotrack-Creator/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

var identityKey = "id"

func authenticator(db *gorm.DB) func(c *gin.Context) (any, error) {
	return func(c *gin.Context) (any, error) {
		var login login
		if err := c.ShouldBindJSON(&login); err != nil {
			return "", jwt.ErrMissingLoginValues
		}

		var user models.User
		if err := db.Preload("Role").Where("username = ?", login.Username).First(&user).Error; err != nil {
			return nil, jwt.ErrFailedAuthentication
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(login.Password)); err != nil {
			return nil, jwt.ErrFailedAuthentication
		}

		return &user, nil
	}
}

func payloadFunc() func(data any) golangjwt.MapClaims {
	return func(data any) golangjwt.MapClaims {
		if v, ok := data.(*models.User); ok {
			return golangjwt.MapClaims{
				identityKey: v.ID,
				"role":      v.Role.Name,
			}
		}
		return golangjwt.MapClaims{}
	}
}

func identityHandler() func(c *gin.Context) any {
	return func(c *gin.Context) any {
		claims := jwt.ExtractClaims(c)

		return map[string]interface{}{
			"id":   claims[identityKey],
			"role": claims["role"],
		}
	}
}

// authorizer enforces role-based access:
//   - admin, editor — full access
//   - operator      — read-only (GET requests only)
func authorizer() func(c *gin.Context, data any) bool {
	return func(c *gin.Context, data any) bool {
		claims, ok := data.(map[string]interface{})
		if !ok {
			return false
		}

		role, ok := claims["role"].(string)
		if !ok {
			return false
		}

		switch role {
		case "admin", "editor":
			return true
		case "operator":
			return c.Request.Method == http.MethodGet
		default:
			return false
		}
	}
}

func unauthorized() func(c *gin.Context, code int, message string) {
	return func(c *gin.Context, code int, message string) {
		c.JSON(code, gin.H{
			"error": message,
		})
	}
}

func logoutResponse() func(c *gin.Context) {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "logout",
		})
	}
}

func initParams(db *gorm.DB, JWTSecret string) *jwt.GinJWTMiddleware {
	return &jwt.GinJWTMiddleware{
		Realm:       "melotrack",
		Key:         []byte(JWTSecret),
		Timeout:     999999 * time.Hour,
		MaxRefresh:  999999 * time.Hour,
		IdentityKey: identityKey,
		PayloadFunc: payloadFunc(),

		IdentityHandler: identityHandler(),
		Authenticator:   authenticator(db),
		Authorizer:      authorizer(),
		Unauthorized:    unauthorized(),
		LogoutResponse:  logoutResponse(),
		TokenLookup:     "header: Authorization, query: token, cookie: jwt",

		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	}
}

func SetupAuthMiddleware(db *gorm.DB, JWTSecret string) (*jwt.GinJWTMiddleware, error) {
	authMiddleware, err := jwt.New(initParams(db, JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("SetupAuthMiddleware(): jwt.New(): %v", err)
	}

	if errInit := authMiddleware.MiddlewareInit(); errInit != nil {
		return nil, errInit
	}

	return authMiddleware, nil
}
