package totp

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
)

const (
	Issuer      = "ForgeC2"
	SecretSize  = 16
	BackupCodeCount = 8
	BackupCodeLength = 6
)

func GenerateSecret() (string, error) {
	buf := make([]byte, SecretSize)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(buf), nil
}

func GenerateQRCodeURL(username, secret string) string {
	issuer := url.QueryEscape(Issuer)
	account := url.QueryEscape(username)
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		issuer, account, secret, issuer)
}

func VerifyCode(secret, code string) bool {
	code = strings.ReplaceAll(code, " ", "")
	code = strings.ReplaceAll(code, "-", "")
	
	_, err := strconv.ParseUint(code, 10, 64)
	if err != nil {
		return false
	}

	opts := totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    6,
	}

	_, err = totp.ValidateCustom(code, secret, time.Now(), opts)
	return err == nil
}

func GenerateBackupCodes() []string {
	codes := make([]string, BackupCodeCount)
	for i := 0; i < BackupCodeCount; i++ {
		buf := make([]byte, 4)
		rand.Read(buf)
		code := fmt.Sprintf("%06d", int64(uint32(buf[0])<<24|uint32(buf[1])<<16|uint32(buf[2])<<8|uint32(buf[3]))%1000000)
		codes[i] = code[:3] + " " + code[3:]
	}
	return codes
}
