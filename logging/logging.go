package logging

import (
	"context"
	"os"
	"time"

	v2config "github.com/aws/aws-sdk-go-v2/config"
	v2credentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	zzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	pgm "github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
)

func SetupLogging(disableCloudwatch bool) (*zzap.Logger, error) {
	fn := zzap.LevelEnablerFunc(func(_ zapcore.Level) bool {
		return true
	})

	consoleOutput := zapcore.Lock(os.Stdout)
	consoleEncoder := zapcore.NewJSONEncoder(zzap.NewProductionEncoderConfig())

	var core zapcore.Core

	key := os.Getenv("AWS_CW_KEY")
	secret := os.Getenv("AWS_CW_SECRET")
	group := os.Getenv("AWS_CW_LOG_GROUP")
	stream, err := os.Hostname()
	if err != nil {
		stream = "undefined"
	}
	region := os.Getenv("AWS_CW_REGION")

	if !disableCloudwatch && key != "" {
		// Load v2 config for validation and future use
		_, err := v2config.LoadDefaultConfig(context.TODO(),
			v2config.WithRegion(region),
			v2config.WithCredentialsProvider(v2credentials.NewStaticCredentialsProvider(key, secret, "")),
		)
		if err != nil {
			return nil, err
		}

		// Create v1 config for compatibility with platform-go-middlewares
		v1Creds := credentials.NewStaticCredentials(key, secret, "")
		v1Config := aws.NewConfig().WithRegion(region).WithCredentials(v1Creds)

		cwLogger, err := pgm.NewBatchingHook(group, stream, v1Config, time.Second*5)

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
