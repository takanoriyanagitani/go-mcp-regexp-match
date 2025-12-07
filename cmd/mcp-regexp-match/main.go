package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	pattest "github.com/takanoriyanagitani/go-mcp-regexp-match"
	"github.com/takanoriyanagitani/go-mcp-regexp-match/pattester/wasi"
	wa0 "github.com/takanoriyanagitani/go-mcp-regexp-match/pattester/wasi/wa0"
)

const (
	defaultPort         = 12030
	readTimeoutSeconds  = 10
	writeTimeoutSeconds = 10
	maxHeaderExponent   = 20
	maxBodyBytes        = 1 * 1024 * 1024 // 1 MiB
)

var (
	port       = flag.Int("port", defaultPort, "port to listen")
	enginePath = flag.String(
		"path2engine",
		"./engine/rs/rs-regexp-wasi/rs-regexp-wasi.wasm",
		"path to the WASM regex engine",
	)
	mem     = flag.Uint("mem", 64, "WASM memory limit in MiB")
	timeout = flag.Uint("timeout", 100, "WASM execution timeout in milliseconds")
)

const wasmPageSizeKiB = 64
const kiBytesInMiByte = 1024
const wasmPagesInMiB = kiBytesInMiByte / wasmPageSizeKiB

// withMaxBodyBytes is a middleware to limit the size of request bodies.
func withMaxBodyBytes(h http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		h.ServeHTTP(w, r)
	})
}

// toClientError converts an internal error to a client-friendly error string.
func toClientError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline exceeded") {
		return "Pattern matching timed out"
	}
	if errors.Is(err, wa0.ErrUuid) {
		return "Engine configuration error"
	}
	if errors.Is(err, wa0.ErrInput) {
		return "Invalid pattern or text input format"
	}
	if errors.Is(err, wa0.ErrOutputJson) {
		return "Engine output error"
	}
	// For any other specific instantiation error, or generic internal errors.
	if errors.Is(err, wa0.ErrInstantiate) {
		return "Engine instantiation failed"
	}

	// Fallback for any unexpected errors, keeping internal details out of client view.
	return "Internal server error"
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	memoryLimitPages := uint32(*mem) * wasmPagesInMiB
	patternTester, cleanup, err := wasi.NewWasiPatternTester(ctx, *enginePath, memoryLimitPages)
	if err != nil {
		log.Printf("failed to create WASI pattern tester: %v\n", err)
		return
	}
	defer func() {
		if err := cleanup(); err != nil {
			log.Printf("failed to cleanup WASI pattern tester: %v\n", err)
		}
	}()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "regexp-match",
		Version: "v0.1.0", // Initial version
		Title:   "Regular Expression Matcher",
	}, nil)

	patternMatchTool := func(ctx context.Context, req *mcp.CallToolRequest, input pattest.UntrustedInput) (
		*mcp.CallToolResult,
		pattest.PatternTestResultDto,
		error,
	) {
		// Create a new context with a timeout for this specific request.
		timeoutCtx, cancelTimeout := context.WithTimeout(ctx, time.Duration(*timeout)*time.Millisecond)
		defer cancelTimeout()

		result := patternTester(timeoutCtx, input)
		if result.Error != nil {
			log.Printf("Error processing pattern='%s' on text='%s': %v", input.Pattern, input.Text, result.Error)
			clientError := toClientError(result.Error)
			return nil, pattest.PatternTestResultDto{
				IsMatch: false,
				Error:   clientError,
			}, nil
		}
		return nil, pattest.PatternTestResultDto{
			IsMatch: result.IsMatch,
			Error:   "",
		}, nil
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:         "regexp-match",
		Title:        "Regular Expression Matcher",
		Description:  "Tool to match text against a regular expression.",
		Meta:         nil,
		Annotations:  nil,
		InputSchema:  nil, // Inferred by AddTool
		OutputSchema: nil, // Inferred by AddTool
	}, patternMatchTool)

	address := fmt.Sprintf(":%d", *port)

	mcpHandler := mcp.NewStreamableHTTPHandler(
		func(req *http.Request) *mcp.Server { return server },
		//nolint:exhaustruct
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	//nolint:exhaustruct
	httpServer := &http.Server{
		Addr:           address,
		Handler:        withMaxBodyBytes(mcpHandler, maxBodyBytes),
		ReadTimeout:    readTimeoutSeconds * time.Second,
		WriteTimeout:   writeTimeoutSeconds * time.Second,
		MaxHeaderBytes: 1 << maxHeaderExponent,
	}

	log.Printf("Ready to start HTTP MCP server. Listening on %s\n", address)
	err = httpServer.ListenAndServe()
	if err != nil {
		log.Printf("Failed to listen and serve: %v\n", err)
		return
	}
}
