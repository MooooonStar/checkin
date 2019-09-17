package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const (
	Host = "https://api.fox.one/api/v2"
)

type contextKey int

const (
	loggerContextKey contextKey = 0
)

func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, logger)
}

func Logger(ctx context.Context) *zap.Logger {
	return ctx.Value(loggerContextKey).(*zap.Logger)
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

type User struct {
	PhoneNumber string `yaml:"phone_number"`
	Password    string `yaml:"password"`
}

type Config struct {
	Users []User `yaml:"users"`
}

func main() {
	ctx := context.Background()
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	ctx = WithLogger(ctx, logger)

	bt, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		Logger(ctx).Fatal("read file", zap.Error(err))
	}

	var cnf Config
	if err := yaml.Unmarshal(bt, &cnf); err != nil {
		Logger(ctx).Fatal("unmarshal", zap.Error(err))
	}

	for _, user := range cnf.Users {
		go func(phone, pwd string) {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				for {
					if err := Do(ctx, phone, pwd); err != nil {
						Logger(ctx).Error("Do error: ", zap.Error(err))
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}
				Logger(ctx).Info("Will check in after 6h")
				select {
				case <-ticker.C:
				}
			}
		}(user.PhoneNumber, user.Password)
	}

	done := make(chan error, 0)
	<-done
}

func Do(ctx context.Context, phoneNumber, password string) error {
	accessToken, err := Login(ctx, phoneNumber, password)
	if err != nil {
		return err
	}

	checkIn, err := CheckIn(ctx, accessToken)
	if err != nil {
		return err
	}
	Logger(ctx).Info("check in: ", zap.Any("checkin", json.RawMessage(checkIn)))

	id, payment, err := CandyBox(ctx, accessToken)
	if err != nil {
		return err
	}

	transfer, err := Transfer(ctx, payment.OpponentId, payment.Amount, payment.Memo, payment.TraceId, accessToken)
	if err != nil {
		return err
	}
	Logger(ctx).Info("transfer:", zap.Any("transfer", json.RawMessage(transfer)))

	//Sleep 3s to avoid read/write inconsistency
	time.Sleep(3 * time.Second)

	_, err = Claim(ctx, id, accessToken)
	if err != nil {
		return err
	}

	info, err := Info(ctx, accessToken)
	if err != nil {
		return err
	}
	Logger(ctx).Info("info:", zap.Any("info", json.RawMessage(info)))
	return nil
}

func Login(ctx context.Context, phoneNumber, password string) (string, error) {
	sum := md5.Sum([]byte("fox." + password))
	body, _ := json.Marshal(map[string]interface{}{
		"phone_number": phoneNumber,
		"password":     string(hex.EncodeToString(sum[:])),
	})

	bt, err := Request("POST", "/account/login", body, "")
	if err != nil {
		return "", err
	}

	var data struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(bt, &data); err != nil {
		return "", err
	}
	return data.AccessToken, nil
}

func CheckIn(ctx context.Context, accessToken string) ([]byte, error) {
	return Request("POST", "/membership/daily-checkin", nil, accessToken)
}

type Payment struct {
	OpponentId string `json:"opponent_id"`
	Amount     string `json:"amount"`
	Memo       string `json:"memo"`
	TraceId    string `json:"trace_id"`
}

func CandyBox(ctx context.Context, accessToken string) (string, *Payment, error) {
	bt, err := Request("GET", "/rewards/candybox", nil, accessToken)
	if err != nil {
		return "", nil, err
	}

	var data []struct {
		ID      string  `json:"id"`
		Payment Payment `json:"payment_info"`
	}

	if err := json.Unmarshal(bt, &data); err != nil {
		return "", nil, err
	}

	if len(data) == 0 {
		return "", nil, errors.New("No items found")
	}
	return data[0].ID, &data[0].Payment, nil
}

func Transfer(ctx context.Context, opponentId, amount, memo, traceId string, accessToken string) ([]byte, error) {
	body, _ := json.Marshal(map[string]string{
		"opponent_id": opponentId,
		"amount":      amount,
		"memo":        memo,
		"trace_id":    traceId,
	})
	return Request("POST", "/membership/transfer", body, accessToken)
}

func Claim(ctx context.Context, id, accessToken string) ([]byte, error) {
	return Request("POST", "/reward/"+id, nil, accessToken)
}

func Info(ctx context.Context, accessToken string) ([]byte, error) {
	return Request("GET", "/membership", nil, accessToken)
}

func Request(method, uri string, body []byte, accessToken string) ([]byte, error) {
	req, err := http.NewRequest(method, Host+uri, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	var data struct {
		Code    int             `json:"code"`
		Message string          `json:"msg"`
		Data    json.RawMessage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.Code != 0 {
		return nil, errors.New(data.Message)
	}

	return data.Data, nil
}
