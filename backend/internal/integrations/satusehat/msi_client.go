package satusehat

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxMSIBody = 8 << 20
	// MaxPageLimit is the documented upper bound ("limit cannot be more than 2000").
	MaxPageLimit = 2000
	// maxTotalPages bounds pagination even if upstream reports an absurd total_page.
	maxTotalPages = 10000
)

// MSIClient reads Master Sarana Index pages for hospitals. It only issues GET requests.
type MSIClient struct {
	endpoint string
	tokens   TokenSource
	http     *http.Client
	// Retries apply to 5xx, network errors and 429 only; 401 gets exactly one refresh.
	maxRetries    int
	maxRetryAfter time.Duration
	backoff       time.Duration
	sleep         func(context.Context, time.Duration) error
	now           func() time.Time
}

func NewMSIClient(baseURL string, tokens TokenSource, client *http.Client) *MSIClient {
	return &MSIClient{endpoint: strings.TrimRight(baseURL, "/") + "/masterdata/v1/mastersaranaindex/mastersarana", tokens: tokens, http: client, maxRetries: 2, maxRetryAfter: 60 * time.Second, backoff: 500 * time.Millisecond, sleep: sleep, now: time.Now}
}
func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// FetchPage returns one page of active hospitals (jenis_sarana=104), optionally limited to one kab/kota.
func (c *MSIClient) FetchPage(ctx context.Context, kabkota string, page, limit int) (HospitalBatch, error) {
	if page < 1 || limit < 1 || limit > MaxPageLimit || (kabkota != "" && !cityCode.MatchString(kabkota)) {
		return HospitalBatch{}, &Error{Op: "msi", Category: CategoryBadRequest}
	}
	q := url.Values{"jenis_sarana": {HospitalJenisSarana}, "status_aktif": {"true"}, "page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(limit)}}
	if kabkota != "" {
		q.Set("kode_kabkota", kabkota)
	}
	target := c.endpoint + "?" + q.Encode()
	refreshed := false
	for attempt, retries := 1, 0; ; attempt++ {
		token, e := c.tokens.AccessToken(ctx)
		if e != nil {
			return HospitalBatch{}, e
		}
		status, header, body, e := c.get(ctx, target, token)
		if errors.Is(e, errOversized) {
			return HospitalBatch{}, &Error{Op: "msi", Category: CategoryInvalid, Status: status, Attempts: attempt}
		}
		if e != nil {
			te := transportError("msi", e, attempt)
			if te.Category == CategoryCanceled || ctx.Err() != nil || retries >= c.maxRetries {
				return HospitalBatch{}, te
			}
			retries++
			if e = c.sleep(ctx, c.delay(retries)); e != nil {
				return HospitalBatch{}, transportError("msi", e, attempt)
			}
			continue
		}
		switch {
		case status == http.StatusOK:
			return c.decode(page, header, body)
		case status == http.StatusUnauthorized && !refreshed:
			c.tokens.Invalidate(token)
			refreshed = true
			continue
		case status == http.StatusTooManyRequests && retries < c.maxRetries:
			wait, ok := parseRetryAfter(header.Get("Retry-After"), c.now())
			if !ok {
				wait = c.delay(retries + 1)
			}
			if wait > c.maxRetryAfter {
				return HospitalBatch{}, &Error{Op: "msi", Category: CategoryRateLimited, Status: status, Attempts: attempt}
			}
			if deadline, has := ctx.Deadline(); has && c.now().Add(wait).After(deadline) {
				return HospitalBatch{}, &Error{Op: "msi", Category: CategoryRateLimited, Status: status, Attempts: attempt}
			}
			retries++
			if e = c.sleep(ctx, wait); e != nil {
				return HospitalBatch{}, transportError("msi", e, attempt)
			}
		case status >= 500 && retries < c.maxRetries:
			retries++
			if e = c.sleep(ctx, c.delay(retries)); e != nil {
				return HospitalBatch{}, transportError("msi", e, attempt)
			}
		default:
			return HospitalBatch{}, &Error{Op: "msi", Category: statusCategory(status), Status: status, Attempts: attempt}
		}
	}
}

// delay is exponential backoff with full jitter on top of the base step.
func (c *MSIClient) delay(retry int) time.Duration {
	step := c.backoff << (retry - 1)
	if step <= 0 {
		return 0
	}
	return step + rand.N(step)
}
func (c *MSIClient) get(ctx context.Context, target, token string) (int, http.Header, []byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if e != nil {
		return 0, nil, nil, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	res, e := c.http.Do(req)
	if e != nil {
		return 0, nil, nil, e
	}
	defer res.Body.Close()
	body, e := readLimited(res.Body, maxMSIBody)
	if e != nil {
		if ctx.Err() != nil {
			return 0, nil, nil, ctx.Err()
		}
		// Oversized bodies are a permanent payload problem, not a transport error worth retrying.
		return res.StatusCode, res.Header, nil, errOversized
	}
	return res.StatusCode, res.Header, body, nil
}

var errOversized = errors.New("response body too large")

func (c *MSIClient) decode(page int, header http.Header, body []byte) (HospitalBatch, error) {
	invalid := &Error{Op: "msi", Category: CategoryInvalid, Status: http.StatusOK}
	if !isJSON(header.Get("Content-Type")) {
		return HospitalBatch{}, invalid
	}
	var r MSIResponse
	if json.Unmarshal(body, &r) != nil {
		return HospitalBatch{}, invalid
	}
	if r.StatusCode != 0 && r.StatusCode != http.StatusOK {
		return HospitalBatch{}, &Error{Op: "msi", Category: statusCategory(r.StatusCode), Status: r.StatusCode}
	}
	if (r.Page != 0 && r.Page != page) || r.TotalPage < 0 || r.TotalPage > maxTotalPages {
		return HospitalBatch{}, invalid
	}
	b := HospitalBatch{Records: r.Data, Page: page, TotalPage: r.TotalPage}
	if b.Records == nil {
		b.Records = []MSIFacility{}
	}
	if len(b.Records) > 0 && page < r.TotalPage {
		next := page + 1
		b.NextPage = &next
	}
	return b, nil
}
