package web

import (
	"CompeteAI/internal/domain"
	"CompeteAI/internal/pkg/ginx"
	"CompeteAI/internal/service"
	ijwt "CompeteAI/internal/web/jwt"
	"errors"
	regexp "github.com/dlclark/regexp2"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

var _ handler = (*UserHandler)(nil)

// UserHandler 定义所有和user相关的路由
type UserHandler struct {
	svc         service.UserService
	emailExp    *regexp.Regexp
	passwordExp *regexp.Regexp
	ijwt.Handler
}

func NewUserHandler(svc service.UserService, cmd redis.Cmdable) *UserHandler {
	const (
		emailRegexPattern    = `^\w+([-+.]\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*$`
		passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
	)

	return &UserHandler{
		svc:         svc,
		emailExp:    regexp.MustCompile(emailRegexPattern, regexp.None),
		passwordExp: regexp.MustCompile(passwordRegexPattern, regexp.None),
		Handler:     ijwt.NewRedisJwtHandler(cmd),
	}
}

func (u *UserHandler) RegisterRoutes(server *gin.Engine) {
	ug := server.Group("/users")
	ug.POST("/signup", u.SignUp)
	ug.POST("/login", u.LogInJWT)
	ug.POST("/logout", u.LogoutJWT)
	ug.POST("/refresh_token", u.RefreshToekn)
}

func (u *UserHandler) SignUp(c *gin.Context) {
	type SignUpReq struct {
		Email           string `json:"email"`
		ConfirmPassword string `json:"confirmPassword"`
		Password        string `json:"password"`
	}

	var req SignUpReq
	// Bind 方法会根据 Content-Type 来解析你的数据到 req 里面
	// 解析错了，就会返回一个 400 的错误
	if err := c.Bind(&req); err != nil {
		log.Println(err)
		return
	}

	ok, err := u.emailExp.MatchString(req.Email)
	if err != nil {
		c.String(http.StatusOK, "系统错误")
		log.Println(err)
		return
	}

	if !ok {
		c.String(http.StatusOK, "你的邮箱格式不正确")
		return
	}

	if req.Password != req.ConfirmPassword {
		c.String(http.StatusOK, "两次输入的密码不一致")
		return
	}

	ok, err = u.passwordExp.MatchString(req.Password)
	if err != nil {
		c.String(http.StatusOK, "系统错误")
		//log.Println(err)
		return
	}

	if !ok {
		c.String(http.StatusOK, "密码必须大于8位，且包含特殊字符")
		return
	}

	err = u.svc.SignUp(c, domain.User{
		Email:    req.Email,
		Password: req.Password,
	})
	if err == service.ErrUserDuplicate {
		c.String(http.StatusOK, "邮箱冲突")
		return
	}
	if err != nil {
		c.String(http.StatusOK, "系统异常")
		return
	}

	c.String(http.StatusOK, "注册成功!")
	return
}

func (u *UserHandler) LogInJWT(c *gin.Context) {
	type LoginReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var req LoginReq
	if err := c.Bind(&req); err != nil {
		return
	}
	user, err := u.svc.Login(c, req.Email, req.Password)
	if errors.Is(err, service.ErrInvalidUserOrPassword) {
		c.String(http.StatusOK, "用户名/密码错误")
		return
	}
	if err != nil {
		c.String(http.StatusOK, "系统错误")
		return
	}

	if err := u.SetLoginToken(c, user.Id); err != nil {
		c.JSON(http.StatusOK, ginx.Result{
			Code: 5,
			Msg:  "系统错误",
			Data: nil,
		})
		return
	}
	c.String(http.StatusOK, "登录成功")
	return
}

func (u *UserHandler) LogoutJWT(c *gin.Context) {
	err := u.ClearToken(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ginx.Result{
			Code: 5,
			Msg:  "系统错误",
		})
		return
	}
	c.JSON(http.StatusOK, ginx.Result{
		Code: 0,
		Msg:  "退出登录成功",
	})
}

func (u *UserHandler) RefreshToekn(c *gin.Context) {
	// 只有这个接口, 拿出来的才是 refresh-token, 其余的都是 access-token
	refresh_token := u.ExtractToken(c)
	var rc ijwt.RefreshClaims
	token, err := jwt.ParseWithClaims(refresh_token, &rc, func(token *jwt.Token) (interface{}, error) {
		return ijwt.RefreshTokenKey, nil
	})
	if err != nil {
	}
	if err != nil || !token.Valid {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	err = u.CheckSession(c, rc.Ssid)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if err := u.SetJwtToken(c, rc.Uid, rc.Ssid); err != nil {
		log.Printf("设置 JWT token 出现错误: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, ginx.Result{
		Msg: "success",
	})
}
