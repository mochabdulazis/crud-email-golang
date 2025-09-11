package internal

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	joonix "github.com/joonix/log"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"

	command_inbound_adapter "prabogo/internal/adapter/inbound/command"
	fiber_inbound_adapter "prabogo/internal/adapter/inbound/fiber"
	rabbitmq_inbound_adapter "prabogo/internal/adapter/inbound/rabbitmq"
	postgres_outbound_adapter "prabogo/internal/adapter/outbound/postgres"
	rabbitmq_outbound_adapter "prabogo/internal/adapter/outbound/rabbitmq"
	redis_outbound_adapter "prabogo/internal/adapter/outbound/redis"
	"prabogo/internal/domain"
	_ "prabogo/internal/migration/postgres"
	outbound_port "prabogo/internal/port/outbound"
	"prabogo/utils"
	"prabogo/utils/activity"
	"prabogo/utils/database"
	"prabogo/utils/log"
	"prabogo/utils/rabbitmq"
	"prabogo/utils/redis"
)

var embedMigrations embed.FS
var databaseDriverList = []string{"postgres"}
var httpDriverList = []string{"fiber"}
var messageDriverList = []string{"rabbitmq"}
var outboundDatabaseDriver string
var outboundMessageDriver string
var outboundCacheDriver string
var inboundHttpDriver string
var inboundMessageDriver string

type App struct {
	ctx    context.Context
	domain domain.Domain
}

func NewApp() *App {
	ctx := activity.NewContext("init")
	ctx = activity.WithClientID(ctx, "system")
	godotenv.Load(".env")
	configureLogging()
	outboundDatabaseDriver = os.Getenv("OUTBOUND_DATABASE_DRIVER")
	outboundMessageDriver = os.Getenv("OUTBOUND_MESSAGE_DRIVER")
	outboundCacheDriver = os.Getenv("OUTBOUND_CACHE_DRIVER")
	inboundHttpDriver = os.Getenv("INBOUND_HTTP_DRIVER")
	inboundMessageDriver = os.Getenv("INBOUND_MESSAGE_DRIVER")
	domain := domain.NewDomain(
		databaseOutbound(ctx),
		messageOutbound(ctx),
		cacheOutbound(ctx),
	)

	return &App{
		ctx:    ctx,
		domain: domain,
	}
}

func (a *App) Run(option string) {
	switch option {
	case "http":
		a.httpInbound()
	case "message":
		a.messageInbound()
	default:
		a.commandInbound()
	}
}

func databaseOutbound(ctx context.Context) outbound_port.DatabasePort {
	if !utils.IsInList(databaseDriverList, outboundDatabaseDriver) {
		log.WithContext(ctx).Fatal("database driver is not supported")
		os.Exit(1)
	}
	db := database.InitDatabase(ctx, outboundDatabaseDriver)

	switch outboundDatabaseDriver {
	case "postgres":
		return postgres_outbound_adapter.NewAdapter(db)
	}
	return nil
}

func messageOutbound(ctx context.Context) outbound_port.MessagePort {
	if !utils.IsInList(messageDriverList, outboundMessageDriver) {
		log.WithContext(ctx).Fatal("message driver is not supported")
		os.Exit(1)
	}

	switch outboundMessageDriver {
	case "rabbitmq":
		rabbitmq.InitMessage()
		return rabbitmq_outbound_adapter.NewAdapter()
	}
	return nil
}

func cacheOutbound(ctx context.Context) outbound_port.CachePort {
	if !utils.IsInList([]string{"redis"}, outboundCacheDriver) {
		log.WithContext(ctx).Fatal("cache driver is not supported")
		os.Exit(1)
	}

	switch outboundCacheDriver {
	case "redis":
		redis.InitDatabase()
		return redis_outbound_adapter.NewAdapter()
	}
	return nil
}

func (a *App) httpInbound() {
	ctx := a.ctx
	if !utils.IsInList(httpDriverList, inboundHttpDriver) {
		log.WithContext(ctx).Fatal("http driver is not supported")
		os.Exit(1)
	}

	switch inboundHttpDriver {
	case "fiber":
		app := fiber.New()
		inboundHttpAdapter := fiber_inbound_adapter.NewAdapter(a.domain)
		fiber_inbound_adapter.InitRoute(ctx, app, inboundHttpAdapter)
		go func() {
			if err := app.Listen(":" + os.Getenv("SERVER_PORT")); err != nil {
				log.WithContext(ctx).Fatalf("failed to listen and serve: %+v", err)
			}
		}()
	}

	ctx, shutdown := context.WithTimeout(ctx, 5*time.Second)
	defer shutdown()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, os.Interrupt)
	<-quit

	log.WithContext(ctx).Info("http server stopped")
}

func (a *App) messageInbound() {
	ctx := a.ctx
	if !utils.IsInList(messageDriverList, inboundMessageDriver) {
		log.WithContext(ctx).Fatal("message driver is not supported")
		os.Exit(1)
	}

	switch inboundMessageDriver {
	case "rabbitmq":
		inboundMessageAdapter := rabbitmq_inbound_adapter.NewAdapter(a.domain)
		rabbitmq_inbound_adapter.InitRoute(ctx, os.Args, inboundMessageAdapter)
	}
}

func (a *App) commandInbound() {
	ctx := a.ctx
	inboundCommandAdapter := command_inbound_adapter.NewAdapter(a.domain)
	command_inbound_adapter.InitRoute(ctx, os.Args, inboundCommandAdapter)
}

func configureLogging() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.AddHook(utils.LogrusSourceContextHook{})

	if os.Getenv("APP_MODE") != "release" {
		logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	} else {
		logrus.SetFormatter(joonix.NewFormatter())
	}
}
