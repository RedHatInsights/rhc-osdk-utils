package logging

import (
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	zzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pgm "github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
)

func SetupLogging() (*zzap.Logger, error) {
	fn := zzap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return true
	})

	consoleOutput := zapcore.Lock(os.Stdout)
	consoleEncoder := zapcore.NewConsoleEncoder(zzap.NewDevelopmentEncoderConfig())
	var core zapcore.Core

	key := os.Getenv("AWS_CW_KEY")
	secret := os.Getenv("AWS_CW_SECRET")
	group := os.Getenv("AWS_CW_LOG_GROUP")
	stream, err := os.Hostname()
	if err != nil {
		stream = "undefined"
	}
	region := os.Getenv("AWS_CW_REGION")

	if key != "" {
		cred := credentials.NewStaticCredentials(key, secret, "")
		cfg := aws.NewConfig().WithRegion(region).WithCredentials(cred)
		cwLogger, err := pgm.NewBatchingHook(group, stream, cfg, time.Second*5)

		if err != nil {
			return nil, err
		}

		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, consoleOutput, fn),
			zapcore.NewCore(consoleEncoder, cwLogger, fn),
		)
	} else {
		core = zapcore.NewTee(
			zapcore.NewCore(consoleEncoder, consoleOutput, fn),
		)
	}

	logger := zzap.New(core)

	return logger, err
}
