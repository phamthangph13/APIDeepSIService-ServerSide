package mailer

import (
	"fmt"
	"html"
)

type TemplateData struct {
	AppName     string
	Name        string
	URL         string
	Email       string
	PlanName    string
	AmountLabel string
	Invoice     string
	Expires     string
}

func ResetPasswordMessage(data TemplateData) Message {
	subject := "Đặt lại mật khẩu DeepSI Agent"
	text := fmt.Sprintf(`Xin chào %s,

Chúng tôi nhận được yêu cầu đặt lại mật khẩu cho tài khoản DeepSI Agent của bạn.

Mở liên kết sau để chọn mật khẩu mới:
%s

Liên kết có hiệu lực trong %s. Nếu bạn không yêu cầu thay đổi này, hãy bỏ qua email.

DeepSI Agent`, data.Name, data.URL, data.Expires)
	return Message{To: data.Email, Subject: subject, Text: text, HTML: layout(data, "Đặt lại mật khẩu", "Chúng tôi nhận được yêu cầu đặt lại mật khẩu cho tài khoản của bạn. Liên kết chỉ có hiệu lực trong "+data.Expires+".", "Chọn mật khẩu mới", data.URL)}
}

func VerifyEmailMessage(data TemplateData) Message {
	subject := "Xác nhận email DeepSI Agent"
	text := fmt.Sprintf(`Xin chào %s,

Cảm ơn bạn đã tạo tài khoản DeepSI Agent. Xác nhận email của bạn tại:
%s

Liên kết có hiệu lực trong %s.

DeepSI Agent`, data.Name, data.URL, data.Expires)
	return Message{To: data.Email, Subject: subject, Text: text, HTML: layout(data, "Xác nhận email", "Một bước cuối để kích hoạt tài khoản và bắt đầu tạo API key.", "Xác nhận email", data.URL)}
}

func WelcomeMessage(data TemplateData) Message {
	subject := "Chào mừng đến DeepSI Agent"
	text := fmt.Sprintf(`Xin chào %s,

Tài khoản DeepSI Agent của bạn đã sẵn sàng. Tạo API key đầu tiên tại:
%s

DeepSI Agent`, data.Name, data.URL)
	return Message{To: data.Email, Subject: subject, Text: text, HTML: layout(data, "Tài khoản đã sẵn sàng", "Tạo API key đầu tiên, dán base URL và gọi endpoint — mất khoảng bốn phút.", "Mở client", data.URL)}
}

func PaymentReceiptMessage(data TemplateData) Message {
	subject := fmt.Sprintf("Thanh toán thành công · %s", data.Invoice)
	text := fmt.Sprintf(`Xin chào %s,

Chúng tôi đã nhận thanh toán cho gói %s.
Số tiền: %s
Mã hóa đơn: %s

Gói dịch vụ đã được kích hoạt. Cảm ơn bạn đã tin dùng DeepSI Agent.

DeepSI Agent`, data.Name, data.PlanName, data.AmountLabel, data.Invoice)
	return Message{To: data.Email, Subject: subject, Text: text, HTML: layout(data, "Thanh toán thành công", fmt.Sprintf("Gói %s đã được kích hoạt. Hóa đơn %s · %s.", data.PlanName, data.Invoice, data.AmountLabel), "Mở client", data.URL)}
}

func layout(data TemplateData, title, body, cta, url string) string {
	appName := data.AppName
	if appName == "" {
		appName = "DeepSI Agent"
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="vi">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;background:#f3efe6;color:#1d241f;font-family:Arial,Helvetica,sans-serif">
  <div style="max-width:560px;margin:0 auto;padding:32px 20px">
    <div style="font-size:12px;letter-spacing:.14em;text-transform:uppercase;color:#5f6b61;margin-bottom:18px">%s / service.deepsi.space</div>
    <div style="background:#fffdf8;border:1px solid #d8d0c0;border-radius:18px;padding:32px">
      <h1 style="font-size:28px;line-height:1.15;margin:0 0 16px;font-weight:700">%s</h1>
      <p style="font-size:16px;line-height:1.6;color:#4d574f;margin:0 0 24px">Xin chào %s,<br>%s</p>
      <a href="%s" style="display:inline-block;background:#1d241f;color:#fffdf8;text-decoration:none;padding:13px 22px;border-radius:999px;font-weight:700">%s</a>
      <p style="font-size:12px;line-height:1.6;color:#7b857c;margin:24px 0 0">Nếu nút không hoạt động, dán liên kết sau vào trình duyệt:<br><span style="word-break:break-all">%s</span></p>
    </div>
    <p style="font-size:11px;line-height:1.6;color:#8a938b;margin:18px 0 0">Bạn nhận được email này vì có hoạt động trên tài khoản DeepSI Agent.</p>
  </div>
</body>
</html>`, html.EscapeString(appName), html.EscapeString(title), html.EscapeString(data.Name), html.EscapeString(body), html.EscapeString(url), html.EscapeString(cta), html.EscapeString(url))
}
