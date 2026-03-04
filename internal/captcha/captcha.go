package captcha

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	validateAPI = "http://gcaptcha4.geetest.com/validate"
	timeout     = 10 * time.Second
)

type ValidateResponse struct {
	Result      string                 `json:"result"`
	Reason      string                 `json:"reason"`
	CaptchaArgs map[string]interface{} `json:"captcha_args"`
	Status      string                 `json:"status"`
	Code        string                 `json:"code"`
	Msg         string                 `json:"msg"`
}

type Validator struct {
	captchaId  string
	privateKey string
	client     *http.Client
}

func NewValidator(captchaId, privateKey string) *Validator {
	return &Validator{
		captchaId:  captchaId,
		privateKey: privateKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (v *Validator) Verify(lotNumber, captchaOutput, passToken, genTime, userIP string) (*ValidateResponse, error) {
	if lotNumber == "" || captchaOutput == "" || passToken == "" || genTime == "" {
		return nil, fmt.Errorf("missing required parameters")
	}

	signToken := v.generateSignToken(lotNumber)

	data := url.Values{}
	data.Set("lot_number", lotNumber)
	data.Set("captcha_output", captchaOutput)
	data.Set("pass_token", passToken)
	data.Set("gen_time", genTime)
	data.Set("sign_token", signToken)
	if userIP != "" {
		data.Set("user_ip", userIP)
	}

	apiURL := fmt.Sprintf("%s?captcha_id=%s", validateAPI, v.captchaId)

	fmt.Printf("[DEBUG] Geetest validate request - captcha_id: %s, lot_number: %s\n", v.captchaId, lotNumber)

	resp, err := v.client.PostForm(apiURL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to call validate API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("[DEBUG] Geetest validate response: %s\n", string(body))

	var result ValidateResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

func (v *Validator) generateSignToken(lotNumber string) string {
	h := hmac.New(sha256.New, []byte(v.privateKey))
	h.Write([]byte(lotNumber))
	return hex.EncodeToString(h.Sum(nil))
}

func (v *Validator) VerifyWithRetry(lotNumber, captchaOutput, passToken, genTime, userIP string, maxRetries int) (*ValidateResponse, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		result, err := v.Verify(lotNumber, captchaOutput, passToken, genTime, userIP)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("verification failed after %d retries: %w", maxRetries, lastErr)
}
