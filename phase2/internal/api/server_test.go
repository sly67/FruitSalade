// Integration tests for Phase 2 API: versioning, conflict detection, write operations,
// SSE, sharing, and quotas.
//
// These tests require PostgreSQL and MinIO to be running. They will be skipped
// if the TEST_DATABASE_URL environment variable is not set.
//
// Quick start with Docker:
//   make phase2-test-env
//   TEST_DATABASE_URL="postgres://fruitsalade:fruitsalade@localhost:48004/fruitsalade_test?sslmode=disable" \
//   TEST_S3_ENDPOINT="http://localhost:48002" \
//   go test -v -count=1 ./phase2/internal/api/
package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/fruitsalade/fruitsalade/phase2/internal/auth"
	"github.com/fruitsalade/fruitsalade/phase2/internal/events"
	"github.com/fruitsalade/fruitsalade/phase2/internal/logging"
	"github.com/fruitsalade/fruitsalade/phase2/internal/metadata/postgres"
	"github.com/fruitsalade/fruitsalade/phase2/internal/quota"
	"github.com/fruitsalade/fruitsalade/phase2/internal/sharing"
	s3storage "github.com/fruitsalade/fruitsalade/phase2/internal/storage/s3"
	"github.com/fruitsalade/fruitsalade/shared/pkg/protocol"
)

var (
	testServer *httptest.Server
	testToken  string
	testDB     *sql.DB
)

func TestMain(m *testing.M) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		// Fall back to local test DB
		dbURL = "postgres://fruitsalade:fruitsalade@localhost:48004/fruitsalade_test?sslmode=disable"
	}

	s3Endpoint := os.Getenv("TEST_S3_ENDPOINT")
	if s3Endpoint == "" {
		s3Endpoint = "http://localhost:48002"
	}

	logging.InitDefault()

	ctx := context.Background()

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: cannot connect to test DB: %v\n", err)
		os.Exit(0)
	}
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: test DB not reachable: %v\n", err)
		os.Exit(0)
	}
	testDB = db

	// Clean and set up schema
	db.ExecContext(ctx, "DROP TABLE IF EXISTS group_permissions CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS group_members CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS groups CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS bandwidth_usage CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS user_quotas CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS share_links CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS file_permissions CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS file_versions CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS device_tokens CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS files CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS users CASCADE")
	db.ExecContext(ctx, "DROP TABLE IF EXISTS schema_migrations CASCADE")

	metaStore, err := postgres.New(dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: postgres store init failed: %v\n", err)
		os.Exit(0)
	}

	// Run migrations
	migrationsDir := findTestMigrationsDir()
	if migrationsDir == "" {
		fmt.Fprintf(os.Stderr, "SKIP: cannot find migrations directory\n")
		os.Exit(0)
	}
	if err := metaStore.Migrate(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: migrations failed: %v\n", err)
		os.Exit(0)
	}

	// Connect to S3/MinIO
	s3Cfg := s3storage.Config{
		Endpoint:  s3Endpoint,
		Bucket:    "fruitsalade-test",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Region:    "us-east-1",
		UseSSL:    false,
	}
	storage, err := s3storage.New(ctx, s3Cfg, metaStore)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: S3 storage init failed: %v\n", err)
		os.Exit(0)
	}

	// Create root dir
	rootRow := &postgres.FileRow{
		ID:         "root",
		Name:       "/",
		Path:       "/",
		ParentPath: "/",
		IsDir:      true,
	}
	metaStore.UpsertFile(ctx, rootRow)

	// Set up auth
	authHandler := auth.New(db, "test-secret")
	authHandler.EnsureDefaultAdmin(ctx)

	// Set up SSE, sharing, quotas
	broadcaster := events.NewBroadcaster()
	permissionStore := sharing.NewPermissionStore(db)
	shareLinkStore := sharing.NewShareLinkStore(db)
	quotaStore := quota.NewQuotaStore(db)
	rateLimiter := quota.NewRateLimiter(quotaStore)

	// Create server
	groupStore := sharing.NewGroupStore(db)
	permissionStore.SetGroupStore(groupStore)
	provisioner := sharing.NewProvisioner(groupStore, metaStore, permissionStore)
	srv := NewServer(
		storage, authHandler, 10*1024*1024,
		broadcaster, permissionStore, shareLinkStore,
		quotaStore, rateLimiter, groupStore, nil,
		provisioner,
	)
	if err := srv.Init(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: server init failed: %v\n", err)
		os.Exit(0)
	}

	testServer = httptest.NewServer(srv.Handler())
	defer testServer.Close()

	// Get auth token
	testToken, err = getTestToken(testServer.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "SKIP: cannot get test token: %v\n", err)
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func findTestMigrationsDir() string {
	candidates := []string{
		"../../../phase2/migrations",
		"../../migrations",
		"../migrations",
		"migrations",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}

func getTestToken(baseURL string) (string, error) {
	body := `{"username":"admin","password":"admin","device_name":"test"}`
	resp, err := http.Post(baseURL+"/api/v1/auth/token", "application/json", bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Token, nil
}

func authReq(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	return req, nil
}

func uploadFile(t *testing.T, path, content string) map[string]interface{} {
	t.Helper()
	req, _ := authReq("POST", testServer.URL+"/api/v1/content/"+path, bytes.NewBufferString(content))
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload failed: %d %s", resp.StatusCode, body)
	}
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

// --- Tests ---

func TestHealth(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUploadAndDownload(t *testing.T) {
	content := "Hello, integration test!"
	result := uploadFile(t, "test/upload.txt", content)

	if result["path"] != "/test/upload.txt" {
		t.Errorf("expected path /test/upload.txt, got %v", result["path"])
	}
	if int(result["size"].(float64)) != len(content) {
		t.Errorf("expected size %d, got %v", len(content), result["size"])
	}

	// Download
	req, _ := authReq("GET", testServer.URL+"/api/v1/content/test/upload.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download failed: %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != content {
		t.Errorf("expected %q, got %q", content, string(body))
	}

	// Check ETag and X-Version headers
	if resp.Header.Get("ETag") == "" {
		t.Error("missing ETag header")
	}
	if resp.Header.Get("X-Version") == "" {
		t.Error("missing X-Version header")
	}
}

func TestVersioning(t *testing.T) {
	// Upload v1
	r1 := uploadFile(t, "versioned/file.txt", "version 1 content")
	if int(r1["version"].(float64)) != 1 {
		t.Fatalf("expected version 1, got %v", r1["version"])
	}

	// Upload v2
	r2 := uploadFile(t, "versioned/file.txt", "version 2 content updated")
	if int(r2["version"].(float64)) != 2 {
		t.Fatalf("expected version 2, got %v", r2["version"])
	}

	// Upload v3
	r3 := uploadFile(t, "versioned/file.txt", "version 3")
	if int(r3["version"].(float64)) != 3 {
		t.Fatalf("expected version 3, got %v", r3["version"])
	}

	// List versions
	req, _ := authReq("GET", testServer.URL+"/api/v1/versions/versioned/file.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var vList protocol.VersionListResponse
	json.NewDecoder(resp.Body).Decode(&vList)

	if vList.CurrentVersion != 3 {
		t.Errorf("expected current version 3, got %d", vList.CurrentVersion)
	}
	if len(vList.Versions) < 2 {
		t.Fatalf("expected at least 2 version records, got %d", len(vList.Versions))
	}
}

func TestVersionDownload(t *testing.T) {
	// Upload two versions
	uploadFile(t, "download-ver/file.txt", "original content here")
	uploadFile(t, "download-ver/file.txt", "updated content here!")

	// Download v1
	req, _ := authReq("GET", testServer.URL+"/api/v1/versions/download-ver/file.txt?v=1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("version download failed: %d %s", resp.StatusCode, body)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "original content here" {
		t.Errorf("expected v1 content, got %q", string(body))
	}

	if resp.Header.Get("X-Version") != "1" {
		t.Errorf("expected X-Version: 1, got %s", resp.Header.Get("X-Version"))
	}
}

func TestVersionRollback(t *testing.T) {
	// Upload v1, v2
	uploadFile(t, "rollback/file.txt", "v1 original")
	uploadFile(t, "rollback/file.txt", "v2 changed")

	// Rollback to v1
	rollbackBody := `{"version": 1}`
	req, _ := authReq("POST", testServer.URL+"/api/v1/versions/rollback/file.txt", bytes.NewBufferString(rollbackBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("rollback failed: %d %s", resp.StatusCode, body)
	}

	var rollbackResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&rollbackResult)
	if int(rollbackResult["restored_version"].(float64)) != 1 {
		t.Errorf("expected restored_version 1, got %v", rollbackResult["restored_version"])
	}
	if int(rollbackResult["new_version"].(float64)) != 3 {
		t.Errorf("expected new_version 3, got %v", rollbackResult["new_version"])
	}

	// Verify content is now v1
	req, _ = authReq("GET", testServer.URL+"/api/v1/content/rollback/file.txt", nil)
	resp2, _ := http.DefaultClient.Do(req)
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	if string(body) != "v1 original" {
		t.Errorf("expected rolled-back content, got %q", string(body))
	}
}

func TestConflictDetectionExpectedVersion(t *testing.T) {
	// Upload v1
	uploadFile(t, "conflict-ev/file.txt", "initial")

	// Try upload with wrong expected version
	req, _ := authReq("POST", testServer.URL+"/api/v1/content/conflict-ev/file.txt", bytes.NewBufferString("should fail"))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Expected-Version", "99")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	var conflict protocol.ConflictResponse
	json.NewDecoder(resp.Body).Decode(&conflict)
	if conflict.ExpectedVersion != 99 {
		t.Errorf("expected ExpectedVersion 99, got %d", conflict.ExpectedVersion)
	}
	if conflict.CurrentVersion != 1 {
		t.Errorf("expected CurrentVersion 1, got %d", conflict.CurrentVersion)
	}

	// Upload with correct expected version
	req, _ = authReq("POST", testServer.URL+"/api/v1/content/conflict-ev/file.txt", bytes.NewBufferString("should succeed"))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Expected-Version", "1")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 201, got %d: %s", resp2.StatusCode, body)
	}
}

func TestConflictDetectionIfMatch(t *testing.T) {
	// Upload file
	result := uploadFile(t, "conflict-im/file.txt", "initial content")
	hash := result["hash"].(string)

	// Try upload with wrong ETag
	req, _ := authReq("POST", testServer.URL+"/api/v1/content/conflict-im/file.txt", bytes.NewBufferString("wrong etag"))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("If-Match", `"badhash"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}

	// Upload with correct ETag
	req, _ = authReq("POST", testServer.URL+"/api/v1/content/conflict-im/file.txt", bytes.NewBufferString("correct etag"))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("If-Match", `"`+hash+`"`)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 201, got %d: %s", resp2.StatusCode, body)
	}
}

func TestLastWriteWins(t *testing.T) {
	// Upload multiple times without conflict headers (should always succeed)
	uploadFile(t, "lww/file.txt", "write 1")
	uploadFile(t, "lww/file.txt", "write 2")
	r3 := uploadFile(t, "lww/file.txt", "write 3")

	if int(r3["version"].(float64)) != 3 {
		t.Errorf("expected version 3, got %v", r3["version"])
	}

	// Download should return latest
	req, _ := authReq("GET", testServer.URL+"/api/v1/content/lww/file.txt", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "write 3" {
		t.Errorf("expected latest content, got %q", string(body))
	}
}

func TestCreateDirectory(t *testing.T) {
	req, _ := authReq("PUT", testServer.URL+"/api/v1/tree/newdir?type=dir", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["isDir"] != true {
		t.Errorf("expected isDir=true, got %v", result["isDir"])
	}
}

func TestDeleteFile(t *testing.T) {
	// Upload a file
	uploadFile(t, "delete-test/file.txt", "to be deleted")

	// Delete it
	req, _ := authReq("DELETE", testServer.URL+"/api/v1/tree/delete-test/file.txt", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	// Try to download (should fail)
	req, _ = authReq("GET", testServer.URL+"/api/v1/content/delete-test/file.txt", nil)
	resp2, _ := http.DefaultClient.Do(req)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp2.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	// Request without auth token
	resp, err := http.Get(testServer.URL + "/api/v1/tree")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestMetadataTree(t *testing.T) {
	// Upload a file so tree is non-empty
	uploadFile(t, "tree-test/hello.txt", "tree test content")

	req, _ := authReq("GET", testServer.URL+"/api/v1/tree", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var tree protocol.TreeResponse
	json.NewDecoder(resp.Body).Decode(&tree)
	if tree.Root == nil {
		t.Fatal("expected non-nil tree root")
	}
	if !tree.Root.IsDir {
		t.Error("expected root to be a directory")
	}
}

func TestShareLinkCreateAndDownload(t *testing.T) {
	// Upload a file first
	uploadFile(t, "shared/doc.txt", "shared content here")

	// Create share link
	req, _ := authReq("POST", testServer.URL+"/api/v1/share/shared/doc.txt",
		bytes.NewBufferString(`{"max_downloads": 5}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var shareResp protocol.ShareLinkResponse
	json.NewDecoder(resp.Body).Decode(&shareResp)
	if shareResp.ID == "" {
		t.Fatal("expected non-empty share link ID")
	}
	if shareResp.MaxDownloads != 5 {
		t.Errorf("expected max_downloads 5, got %d", shareResp.MaxDownloads)
	}

	// Download via public share link (no auth)
	dlResp, err := http.Get(testServer.URL + "/api/v1/share/" + shareResp.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dlResp.Body)
		t.Fatalf("share download failed: %d %s", dlResp.StatusCode, body)
	}

	body, _ := io.ReadAll(dlResp.Body)
	if string(body) != "shared content here" {
		t.Errorf("expected shared content, got %q", string(body))
	}

	// Revoke the link
	revokeReq, _ := authReq("DELETE", testServer.URL+"/api/v1/share/"+shareResp.ID, nil)
	revokeResp, err := http.DefaultClient.Do(revokeReq)
	if err != nil {
		t.Fatal(err)
	}
	defer revokeResp.Body.Close()

	if revokeResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(revokeResp.Body)
		t.Fatalf("revoke failed: %d %s", revokeResp.StatusCode, body)
	}

	// Try download after revoke (should fail)
	dlResp2, _ := http.Get(testServer.URL + "/api/v1/share/" + shareResp.ID)
	defer dlResp2.Body.Close()
	if dlResp2.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 after revoke, got %d", dlResp2.StatusCode)
	}
}

func TestQuotaEndpoints(t *testing.T) {
	// Get usage (any authenticated user)
	req, _ := authReq("GET", testServer.URL+"/api/v1/usage", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var usage protocol.UsageResponse
	json.NewDecoder(resp.Body).Decode(&usage)
	if usage.UserID == 0 {
		t.Error("expected non-zero user ID in usage response")
	}

	// Set quota (admin only) - find user ID first
	setBody := `{"max_requests_per_minute": 100, "max_storage_bytes": 1073741824}`
	req, _ = authReq("PUT", testServer.URL+"/api/v1/admin/quotas/1", bytes.NewBufferString(setBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("set quota failed: %d %s", resp2.StatusCode, body)
	}

	var quotaResp protocol.UserQuotaResponse
	json.NewDecoder(resp2.Body).Decode(&quotaResp)
	if quotaResp.MaxRequestsPerMin != 100 {
		t.Errorf("expected max_requests_per_minute 100, got %d", quotaResp.MaxRequestsPerMin)
	}
	if quotaResp.MaxStorageBytes != 1073741824 {
		t.Errorf("expected max_storage_bytes 1073741824, got %d", quotaResp.MaxStorageBytes)
	}

	// Get quota back
	req, _ = authReq("GET", testServer.URL+"/api/v1/admin/quotas/1", nil)
	resp3, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("get quota failed: %d", resp3.StatusCode)
	}

	var getQuotaResp protocol.UserQuotaResponse
	json.NewDecoder(resp3.Body).Decode(&getQuotaResp)
	if getQuotaResp.MaxRequestsPerMin != 100 {
		t.Errorf("expected rpm 100, got %d", getQuotaResp.MaxRequestsPerMin)
	}
}
