package types_test

import (
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestMessage_BeforeCreate_SeqNumPerInvokeID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:message_seq?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Message{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	for want := uint(0); want < 3; want++ {
		msg := types.Message{InvokeID: "inv-1", Text: "x", From: providers.Role_Master}
		if err := db.Create(&msg).Error; err != nil {
			t.Fatalf("Create #%d: %v", want, err)
		}
		if msg.SeqNum != want {
			t.Fatalf("inv-1 seq_num: want %d, got %d", want, msg.SeqNum)
		}
	}

	other := types.Message{InvokeID: "inv-2", Text: "y", From: providers.Role_Master}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("Create inv-2: %v", err)
	}
	if other.SeqNum != 0 {
		t.Fatalf("inv-2 must restart at 0, got %d", other.SeqNum)
	}
}
