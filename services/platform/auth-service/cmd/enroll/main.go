// enroll is an operator-only credential migration tool. No public registration
// endpoint can select an existing ID or assign roles.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/askxuan/auth-service/internal/config"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
)

func main() {
	file := flag.String("f", "etc/auth.yaml", "auth configuration path")
	domain := flag.String("domain", "", "user or admin")
	id := flag.Int64("id", 0, "existing verified business account ID")
	email := flag.String("email", "", "account holder email")
	username := flag.String("username", "", "unique login username")
	send := flag.Bool("send", false, "send an enrollment email; otherwise read code and password from stdin")
	flag.Parse()
	var c config.Config
	conf.MustLoad(*file, &c)
	// Never log SQL parameters, hashes, codes or credentials from this tool.
	logx.Disable()
	s := identity.Accounts{DB: sqlx.NewMysql(c.MySQL.DataSource), Challenges: &identity.Challenges{Redis: redis.MustNewRedis(c.Redis), Secret: c.Auth.AccessSecret}, Mailer: identity.SMTPFromEnv()}
	code, password := "", ""
	if !*send {
		scan := bufio.NewScanner(os.Stdin)
		if !scan.Scan() {
			fmt.Fprintln(os.Stderr, "stdin requires code then password on separate lines")
			os.Exit(1)
		}
		code = strings.TrimSpace(scan.Text())
		if !scan.Scan() {
			fmt.Fprintln(os.Stderr, "stdin requires password")
			os.Exit(1)
		}
		password = scan.Text()
	}
	if err := s.Enroll(context.Background(), *domain, *id, *email, *username, password, code, *send); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("操作完成；请使用新凭据登录验证。")
}
