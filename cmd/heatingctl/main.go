package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"empirebus-tests/heating"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "ensure-on":
		runEnsureOn(os.Args[2:])
	case "get-target-temp":
		runGetTargetTemp(os.Args[2:])
	case "set-target-temp":
		runSetTargetTemp(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: heatingctl <ensure-on|get-target-temp|set-target-temp> [flags]")
}

func sessionFromFlags(fs *flag.FlagSet, args []string) (*heating.Session, context.Context) {
	wsURL := fs.String("ws-url", heating.DefaultWSURL, "websocket URL")
	origin := fs.String("origin", "", "origin header")
	timeout := fs.Duration("timeout", 30*time.Second, "operation timeout")
	heartbeat := fs.Duration("heartbeat-interval", 4*time.Second, "heartbeat interval")
	verbose := fs.Bool("verbose", false, "print relevant heating frames")
	traceWindow := fs.Duration("trace-window", 3*time.Second, "verbose trace window")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	go func() {
		<-ctx.Done()
		stop()
	}()
	_ = stop

	logger := log.New(os.Stdout, "", 0)
	session := heating.NewSession(heating.SessionConfig{
		WSURL:             *wsURL,
		Origin:            *origin,
		HeartbeatInterval: *heartbeat,
		Verbose:           *verbose,
		TraceWindow:       *traceWindow,
		Logger:            logger,
	})
	opCtx, cancel := context.WithTimeout(ctx, *timeout)
	go func() {
		<-opCtx.Done()
		cancel()
	}()
	return session, opCtx
}

func runEnsureOn(args []string) {
	fs := flag.NewFlagSet("ensure-on", flag.ExitOnError)
	session, ctx := sessionFromFlags(fs, args)
	if err := session.Connect(ctx); err != nil {
		fail(err)
	}
	defer session.Close()
	client := heating.NewClient(session)
	if err := client.EnsureOn(ctx); err != nil {
		fail(err)
	}
	fmt.Println(client.State().String())
}

func runGetTargetTemp(args []string) {
	fs := flag.NewFlagSet("get-target-temp", flag.ExitOnError)
	session, ctx := sessionFromFlags(fs, args)
	if err := session.Connect(ctx); err != nil {
		fail(err)
	}
	defer session.Close()
	client := heating.NewClient(session)
	temp, err := client.GetTargetTemp(ctx)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%.1f\n", temp)
}

func runSetTargetTemp(args []string) {
	fs := flag.NewFlagSet("set-target-temp", flag.ExitOnError)
	target := fs.Float64("value", -1000, "target temperature in C")
	session, ctx := sessionFromFlags(fs, args)
	if *target == -1000 {
		fail(fmt.Errorf("--value is required"))
	}
	if err := session.Connect(ctx); err != nil {
		fail(err)
	}
	defer session.Close()
	client := heating.NewClient(session)
	if err := client.SetTargetTemp(ctx, *target); err != nil {
		fail(err)
	}
	fmt.Println(client.State().String())
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
