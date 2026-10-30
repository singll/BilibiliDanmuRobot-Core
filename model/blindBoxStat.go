package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type (
	BlindBoxStatModel interface {
		Insert(ctx context.Context, tx *gorm.DB, data *BlindBoxStatBase) error
		GetTotalOnePersion(ctx context.Context, uid int64, year, month, day int16) (*Result, error)
		GetTotal(ctx context.Context, year, month, day int16) (*Result, error)
		// A nil uid selects the whole room; zero bounds select all recorded dates.
		GetTotalInRange(ctx context.Context, uid *int64, start, end time.Time) (*Result, error)
	}
	defaultBlindBoxStatModel struct {
		conn  *gorm.DB
		table string
	}
	BlindBoxStatBase struct {
		ID                int64 `gorm:"primaryKey;autoIncrement"`
		Uid               int64
		BlindBoxName      string
		Price             int32 // 爆出的礼物价格
		OriginalGiftPrice int32 // 原始盲盒价格
		Cnt               int32
		Year              int16
		Month             int16
		Day               int16
	}

	Result struct {
		C int
		R int64
	}
)

func NewBlindBoxStatModel(conn *gorm.DB, RoomID int64) BlindBoxStatModel {
	err := conn.Table(fmt.Sprintf("blind_%v", RoomID)).AutoMigrate(&BlindBoxStatBase{})
	if err != nil {
		logx.Error(err)
	}
	return &defaultBlindBoxStatModel{
		conn:  conn,
		table: fmt.Sprintf("blind_%v", RoomID),
	}
}

func (m *defaultBlindBoxStatModel) Insert(ctx context.Context, tx *gorm.DB, data *BlindBoxStatBase) error {
	db := m.conn
	if tx != nil {
		db = tx
	}
	err := db.WithContext(ctx).Table(m.table).Save(&data).Error
	return err
}

// GetTotalInRange includes start and excludes end. Comparing calendar dates keeps
// existing records usable and correctly handles weeks crossing months or years.
func (m *defaultBlindBoxStatModel) GetTotalInRange(ctx context.Context, uid *int64, start, end time.Time) (*Result, error) {
	var resp Result
	d := m.conn.WithContext(ctx).Table(m.table).
		Select("COALESCE(SUM(cnt), 0) AS c, COALESCE(SUM(cnt * (price - original_gift_price)), 0) AS r")
	if uid != nil {
		d = d.Where("uid = ?", *uid)
	}
	dateKey := func(t time.Time) int { return t.Year()*10000 + int(t.Month())*100 + t.Day() }
	if !start.IsZero() {
		d = d.Where("(year * 10000 + month * 100 + day) >= ?", dateKey(start))
	}
	if !end.IsZero() {
		d = d.Where("(year * 10000 + month * 100 + day) < ?", dateKey(end))
	}
	if err := d.Take(&resp).Error; err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *defaultBlindBoxStatModel) GetTotalOnePersion(ctx context.Context, uid int64, year, month, day int16) (*Result, error) {
	var resp Result

	d := m.conn.WithContext(ctx).Table(m.table).Model(&BlindBoxStatBase{}).Select(`sum(cnt) as C, (sum(cnt*Price)-sum(cnt*original_gift_price)) as R`).Where("uid = ?", uid)
	if year > 0 {
		d = d.Where("year = ?", year)
	}
	if month > 0 {
		d = d.Where("month = ?", month)
	}
	if day > 0 {
		d = d.Where("day = ?", day)
	}
	err := d.Take(&resp).Error

	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *defaultBlindBoxStatModel) GetTotal(ctx context.Context, year, month, day int16) (*Result, error) {
	var resp Result

	d := m.conn.WithContext(ctx).Table(m.table).Model(&BlindBoxStatBase{}).Select(`sum(cnt) as C, (sum(cnt*Price)-sum(cnt*original_gift_price)) as R`)
	if year > 0 {
		d = d.Where("year = ?", year)
	}
	if month > 0 {
		d = d.Where("month = ?", month)
	}
	if day > 0 {
		d = d.Where("day = ?", day)
	}
	err := d.Take(&resp).Error

	switch err {
	case nil:
		return &resp, nil
	case ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
