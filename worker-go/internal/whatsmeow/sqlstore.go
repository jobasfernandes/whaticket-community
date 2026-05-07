package whatsmeow

import (
	"context"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"

	"github.com/jobasfernandes/whaticket-go-worker/internal/config"
)

func OpenContainer(ctx context.Context, cfg *config.Config, log waLog.Logger) (*sqlstore.Container, error) {
	dsn := cfg.SQLStorePath + "?_pragma=foreign_keys(1)"
	return sqlstore.New(ctx, "sqlite", dsn, log)
}

func SetGlobalDeviceProps(cfg *config.Config) {
	store.DeviceProps.PlatformType = mapPlatformType(cfg.PlatformType)
	osName := cfg.OSName
	store.DeviceProps.Os = &osName
}

func NewClientForDevice(device *store.Device, log waLog.Logger) *whatsmeow.Client {
	return whatsmeow.NewClient(device, log)
}

func mapPlatformType(name string) *waCompanionReg.DeviceProps_PlatformType {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "CHROME":
		return waCompanionReg.DeviceProps_CHROME.Enum()
	case "FIREFOX":
		return waCompanionReg.DeviceProps_FIREFOX.Enum()
	case "SAFARI":
		return waCompanionReg.DeviceProps_SAFARI.Enum()
	case "EDGE":
		return waCompanionReg.DeviceProps_EDGE.Enum()
	case "OPERA":
		return waCompanionReg.DeviceProps_OPERA.Enum()
	case "DESKTOP":
		return waCompanionReg.DeviceProps_DESKTOP.Enum()
	case "IPAD":
		return waCompanionReg.DeviceProps_IPAD.Enum()
	case "ANDROID_TABLET":
		return waCompanionReg.DeviceProps_ANDROID_TABLET.Enum()
	case "IOS_PHONE", "IOS":
		return waCompanionReg.DeviceProps_IOS_PHONE.Enum()
	case "ANDROID_PHONE", "ANDROID":
		return waCompanionReg.DeviceProps_ANDROID_PHONE.Enum()
	default:
		return waCompanionReg.DeviceProps_DESKTOP.Enum()
	}
}
