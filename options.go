package easygpool

import (
	"log"
	"os"
	"time"
)

type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

var (
	logLmsgprefix = 64
	defaultLogger = Logger(log.New(os.Stderr, "[EasyGPool]: ", log.LstdFlags|logLmsgprefix|log.Lmicroseconds))
)

type Options struct {

	// 禁用清理
	DisablePurge bool

	// 被清理掉协程，最大空闲时长超过 MaxIdleTime 就会被清理掉
	MaxIdleTime time.Duration

	// NonBlocking = true非阻塞模式
	// NonBlocking = false 阻塞模式，下面的MaxBlockingTasks参数才有作用
	NonBlocking bool

	// 阻塞模式：当NonBlocking=false的时候，最多阻塞的协程数 MaxBlockingTasks， MaxBlockingTasks = 0表示可以无限阻塞
	MaxBlockingTasks int32

	// 日志打印器
	Logger Logger

	// 协程panic的时候调用
	PanicHandler func(any)
}

type Option func(opt *Options)

func SetDisablePurge(disablePurge bool) Option {
	return func(opt *Options) {
		opt.DisablePurge = disablePurge
	}
}

func SetMaxIdleTime(duration time.Duration) Option {
	return func(opt *Options) {
		opt.MaxIdleTime = duration
	}
}

func SetNonBlocking(nonBlocking bool) Option {
	return func(opt *Options) {
		opt.NonBlocking = nonBlocking
	}
}

func SetMaxBlockingTasks(maxBlockingTasks int32) Option {
	return func(opt *Options) {
		opt.MaxBlockingTasks = maxBlockingTasks
	}
}

func SetLogger(logger Logger) Option {
	return func(opt *Options) {
		opt.Logger = logger
	}
}

func SetPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}
