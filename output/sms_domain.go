package dao

import (
	"time"

	gus "gopay/util/seaenum"
)

// 短信列表
type SMS struct {
	ID          uint64       `json:"id"`                                                  // 流水號
	Currency    gus.Currency `json:"currency"`                                            // 幣別
	PhoneNumber string       `json:"phone_number"`                                        // 門號
	Msg         string       `json:"msg"`                                                 // 內容
	CreateTime  time.Time    `json:"create_time" time_format:"2006-01-02T15:04:05Z07:00"` // 建立時間
}

func (SMS) TableName() string {
	return "sms"
}
