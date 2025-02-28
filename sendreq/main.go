package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
)

var (
	host, path, method string = "localhost", "/", http.MethodGet
	port               int    = 8080
)

type Header struct {
	Key, Value string
}

type Response struct {
	Headers    []Header
	Body       string
	StatusCode int
}

func (resp *Response) WithHeader(key, value string) *Response {
	resp.Headers = append(resp.Headers, Header{AsTitle(key), value})
	return resp
}

func (resp *Response) WriteTo(w io.Writer) (n int64, err error) {
	printf := func(format string, args ...any) error {
		m, err := fmt.Fprintf(w, format, args...)
		n += int64(m)
		return err
	}
	if err := printf("HTTP/1.1 %d %s\r\n", resp.StatusCode, http.StatusText(resp.StatusCode)); err != nil {
		return n, err
	}
	for _, h := range resp.Headers {
		if err := printf("%s: %s\r\n", h.Key, h.Value); err != nil {
			return n, err
		}

	}
	if err := printf("\r\n%s\r\n", resp.Body); err != nil {
		return n, err
	}
	return n, nil
}

func (resp *Response) String() string {
	b := new(strings.Builder)
	resp.WriteTo(b)
	return b.String()
}

func (resp *Response) MarshalText() ([]byte, error) {
	b := new(bytes.Buffer)
	resp.WriteTo(b)
	return b.Bytes(), nil
}

type Request struct {
	Headers            []Header
	Method, Path, Body string
}

func (r *Request) WithHeader(key, value string) *Request {
	r.Headers = append(r.Headers, Header{AsTitle(key), value})
	return r
}

func (r *Request) WriteTo(w io.Writer) (n int64, err error) {
	// write & count bytes written.
	// using small closures like this to cut down on repetition
	// can be nice; but you sometimes pay a performance penalty.
	printf := func(format string, args ...any) error {
		m, err := fmt.Fprintf(w, format, args...)
		n += int64(m)
		return err
	}
	// remember, a HTTP request looks like this:
	// <METHOD>  <PATH>  <PROTOCOL/VERSION>
	// <HEADER>: <VALUE>
	// <HEADER>: <VALUE>
	//
	// <REQUEST BODY>

	// write the request line: like "GET /index.html HTTP/1.1"
	if err := printf("%s %s HTTP/1.1\r\n", r.Method, r.Path); err != nil {
		return n, err
	}

	// write the headers. we don't do anything to order them or combine/merge duplicate headers; this is just an example.
	for _, h := range r.Headers {
		if err := printf("%s: %s\r\n", h.Key, h.Value); err != nil {
			return n, err
		}
	}
	printf("\r\n")                 // write the empty line that separates the headers from the body
	err = printf("%s\r\n", r.Body) // write the body and terminate with a newline
	return n, err
}

func (r *Request) String() string {
	b := new(strings.Builder)
	r.WriteTo(b)
	return b.String()
}
func (r *Request) MarshalText() ([]byte, error) {
	b := new(bytes.Buffer)
	r.WriteTo(b)
	return b.Bytes(), nil
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	const name = "sendreq"
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	log = log.With("app", name)
	slog.SetDefault(log)

	flag.StringVar(&method, "method", method, "http method to use")
	flag.StringVar(&host, "host", host, "host to connect to")
	flag.StringVar(&path, "path", path, "path to request")
	flag.IntVar(&port, "port", port, "port to connect to")
	flag.Parse()

	ip, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		slog.ErrorContext(ctx, "main", "error resolving tcp address", err.Error())
		os.Exit(1)
	}

	conn, err := net.DialTCP("tcp", nil, ip)
	if err != nil {
		slog.ErrorContext(ctx, "main", "error dialing tcp address", err.Error())
		os.Exit(1)
	}
	defer conn.Close()
	slog.InfoContext(ctx, "main", "message", fmt.Sprintf("connected to %s (@ %s)", host, conn.RemoteAddr()))

	reqFields := []string{
		fmt.Sprintf("%s %s HTTP/1.1", method, path), // request line
		"Host: " + host,
		"User-Agent: httpget",
		"", // empty line to terminate the headers
	}
	request := strings.Join(reqFields, "\r\n") + "\r\n"

	exit := func(err error) {
		conn.Close()
		if err != nil {
			slog.ErrorContext(ctx, "main", "error", err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}

	go func() {
		<-ctx.Done()
		exit(nil)
	}()

	_, err = conn.Write([]byte(request))
	if err != nil {
		exit(err)
	}
	slog.InfoContext(ctx, "main", "info", fmt.Sprintf("sent request:\n%s", request))

	for scanner := bufio.NewScanner(conn); scanner.Scan(); {
		line := scanner.Bytes()
		if _, err := fmt.Fprintf(os.Stdout, "%s\n", line); err != nil {
			slog.ErrorContext(ctx, "main", "error writing to connection", err.Error())
		}

		if err := scanner.Err(); err != nil {
			slog.ErrorContext(ctx, "main", "error reading from connection", err.Error())
			return
		}
	}
}

func NewRequest(method, path, host, body string) (*Request, error) {
	switch {
	case method == "":
		return nil, errors.New("missing required argument: method")
	case path == "":
		return nil, errors.New("missing required argument: path")
	case !strings.HasPrefix(path, "/"):
		return nil, errors.New("path must start with /")
	case host == "":
		return nil, errors.New("missing required argument: host")
	default:
		headers := make([]Header, 2)
		headers[0] = Header{"Host", host}
		if body != "" {
			headers = append(headers, Header{"Content-Length", fmt.Sprintf("%d", len(body))})
		}
		return &Request{Method: method, Path: path, Headers: headers, Body: body}, nil
	}
}

func NewResponse(status int, body string) (*Response, error) {
	switch {
	case status < 100 || status > 599:
		return nil, errors.New("invalid status code")
	default:
		if body == "" {
			body = http.StatusText(status)
		}
		headers := []Header{{"Content-Length", fmt.Sprintf("%d", len(body))}}
		return &Response{
			StatusCode: status,
			Headers:    headers,
			Body:       body,
		}, nil
	}
}

// AsTitle returns the given header key as title case; e.g. "content-type" -> "Content-Type"
// It will panic if the key is empty.
func AsTitle(key string) string {
	/* design note --- an empty string could be considered 'in title case',
	   but in practice it's probably programmer error. rather than guess, we'll panic.
	*/
	if key == "" {
		panic("empty header key")
	}

	if isTitleCase(key) {
		return key
	}

	/* ---- design note: allocation is very expensive, while iteration through strings is very cheap.
	   in general, better to check twice rather than allocate once. ----
	*/
	return newTitleCase(key)
}

// newTitleCase returns the given header key as title case; e.g. "content-type" -> "Content-Type";
// it always allocates a new string.
func newTitleCase(key string) string {
	var b strings.Builder
	b.Grow(len(key))
	for i := range key {

		if i == 0 || key[i-1] == '-' {
			b.WriteByte(upper(key[i]))
		} else {
			b.WriteByte(lower(key[i]))
		}
	}
	return b.String()
}

// straight from K&R C, 2nd edition, page 43. some classics never go out of style.
func lower(c byte) byte {
	/* if you're having trouble understanding this:
	   the idea is as follows: A..=Z are 65..=90, and a..=z are 97..=122.
	   so upper-case letters are 32 less than their lower-case counterparts (or 'a'-'A' == 32).
	   rather than using the 'magic' number 32, we use 'a'-'A' to get the same result.
	*/
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c + 'A' - 'a'
	}
	return c
}

// isTitleCase returns true if the given header key is already title case; i.e, it is of the form "Content-Type" or "Content-Length", "Some-Odd-Header", etc.
func isTitleCase(key string) bool {
	// check if this is already title case.
	for i := range key {
		if i == 0 || key[i-1] == '-' {
			if key[i] >= 'a' && key[i] <= 'z' {
				return false
			}
		} else if key[i] >= 'A' && key[i] <= 'Z' {
			return false
		}
	}
	return true
}

func ParseRequest(raw string) (r Request, err error) {
	// request has three parts:
	// 1. Request line
	// 2. Headers
	// 3. Body (optional)

	lines := strings.Split(raw, "\r\n")
	if len(lines) < 3 {
		return Request{}, fmt.Errorf("malformed request: should have at least 3 lines")
	}

	// the request line
	first := strings.Fields(lines[0])
	if len(first) < 3 {
		return Request{}, fmt.Errorf("malformed request line: should have at least 3 lines")
	}

	var protocol string
	r.Method, r.Path, protocol = first[0], first[1], first[2]
	if !strings.HasPrefix(r.Path, "/") {
		return Request{}, fmt.Errorf("malformed request: path should start with /")
	}
	if !strings.Contains(protocol, "HTTP") {
		return Request{}, fmt.Errorf("malformed request: first line should contain HTTP version")
	}

	foundHost := false
	bodyStart := 0

	// handle headers
	for i := 1; i < len(lines); i++ {
		if lines[i] == "" {
			bodyStart = i + 1
			break
		}

		k, v, ok := strings.Cut(lines[i], ": ")
		if !ok {
			return Request{}, fmt.Errorf("malformed request: header %q should be of form 'key: value'", lines[i])
		}

		if strings.ToLower(k) == "host" {
			foundHost = true
		}

		k = AsTitle(k)
		r.Headers = append(r.Headers, Header{Key: k, Value: v})
	}

	if !foundHost {
		return Request{}, fmt.Errorf("malformed request: missing Host header")
	}

	end := len(lines) - 1
	r.Body = strings.Join(lines[bodyStart:end], "\r\n") // go upto but not including last empty line

	return r, nil
}

// ParseResponse parses the given HTTP/1.1 response string into the Response. It returns an error if the Response is invalid,
// - not a valid integer
// - invalid status code
// - missing status text
// - invalid headers
// it doesn't properly handle multi-line headers, headers with multiple values, or html-encoding, etc.
func ParseResponse(raw string) (r *Response, err error) {
	// response has three parts:
	// 1. Response line
	// 2. Headers
	// 3. Body (optional)

	lines := strings.Split(raw, "\r\n")
	if len(lines) < 3 {
		return r, fmt.Errorf("malformed response: should have at least 3 lines")
	}

	responseLine := strings.SplitN(lines[0], " ", 3)
	if len(responseLine) < 3 {
		return r, fmt.Errorf("malformed response line: should have at least 3 lines")
	}

	protocol, statusCode, statusText := responseLine[0], responseLine[1], responseLine[2]
	if !strings.Contains(protocol, "HTTP") {
		return nil, fmt.Errorf("malformed response: first line should contain HTTP version")
	}

	r = new(Response)
	r.StatusCode, err = strconv.Atoi(statusCode)
	if err != nil {
		return nil, fmt.Errorf("malformed response: expected status code to be an integer, got %q", statusCode)
	}

	if statusText == "" || http.StatusText(r.StatusCode) != statusText {
		log.Printf("missing or incorrect status text for status code %d: expected %q, but got %q", r.StatusCode, http.StatusText(r.StatusCode), statusText)
	}

	var bodyStart int
	// then we have headers, up until an empty line.
	for i := 1; i < len(lines); i++ {
		log.Println(i, lines[i])
		if lines[i] == "" { // empty line
			bodyStart = i + 1
			break
		}
		key, val, ok := strings.Cut(lines[i], ": ")
		if !ok {
			return nil, fmt.Errorf("malformed response: header %q should be of form 'key: value'", lines[i])
		}
		key = AsTitle(key)
		r.Headers = append(r.Headers, Header{key, val})
	}
	r.Body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\r\n")) // recombine the body using normal newlines.
	return r, nil
}
