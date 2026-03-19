package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NotFoundError signals a 404 or missing resource.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

// APIError represents an error response from the Komodo API.
type APIError struct {
	Error string   `json:"error"`
	Trace []string `json:"trace,omitempty"`
}

// Client wraps the Komodo Core API.
type Client struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
	UserAgent  string
}

// NewClient constructs a Komodo API client.
func NewClient(baseURL, apiKey, apiSecret, userAgent string) *Client {
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		UserAgent: userAgent,
	}
}

// rpcRequest is the common body shape for Komodo RPC calls.
type rpcRequest struct {
	Type   string      `json:"type"`
	Params interface{} `json:"params"`
}

// doRPC sends an RPC request to the given path (/read, /write, /execute).
func (c *Client) doRPC(ctx context.Context, path string, reqType string, params interface{}, result interface{}) error {
	body := rpcRequest{
		Type:   reqType,
		Params: params,
	}

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshalling request body: %w", err)
	}

	// Retry loop for transient errors.
	var resp *http.Response
	maxRetries := 3
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", c.APIKey)
		req.Header.Set("X-Api-Secret", c.APISecret)
		req.Header.Set("User-Agent", c.UserAgent)

		resp, err = c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			// Only close the body when we are going to retry, otherwise leave it open for the deferred close below.
			if attempt < maxRetries {
				resp.Body.Close()
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
		}
		break
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return &NotFoundError{Message: "resource not found"}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr APIError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, apiErr.Error)
		}
		// Truncate raw body to avoid leaking sensitive data from unexpected responses.
		msg := string(respBody)
		if len(msg) > 200 {
			msg = msg[:200] + "…"
		}
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, msg)
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("unmarshalling response: %w", err)
		}
	}

	return nil
}

// Read sends a request to the /read endpoint.
func (c *Client) Read(ctx context.Context, reqType string, params interface{}, result interface{}) error {
	return c.doRPC(ctx, "/read", reqType, params, result)
}

// Write sends a request to the /write endpoint.
func (c *Client) Write(ctx context.Context, reqType string, params interface{}, result interface{}) error {
	return c.doRPC(ctx, "/write", reqType, params, result)
}

// Execute sends a request to the /execute endpoint.
func (c *Client) Execute(ctx context.Context, reqType string, params interface{}, result interface{}) error {
	return c.doRPC(ctx, "/execute", reqType, params, result)
}

// ---- Common types ----

// parseMongoID extracts the hex string from a Komodo API ID field.
// The API returns IDs as MongoDB ObjectId objects: {"$oid": "hexstring"}.
// It also handles the case where the value is already a plain string.
func parseMongoID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var oid struct {
		OID string `json:"$oid"`
	}
	if json.Unmarshal(raw, &oid) == nil {
		return oid.OID
	}
	return ""
}

// Resource is the common base for all Komodo resources.
type Resource struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Tags   []string        `json:"tags,omitempty"`
	Config json.RawMessage `json:"config,omitempty"`
	Info   json.RawMessage `json:"info,omitempty"`
}

// ---- Server ----

type ServerConfig struct {
	Address                   string   `json:"address,omitempty"`
	ExternalAddress           string   `json:"external_address,omitempty"`
	Region                    string   `json:"region,omitempty"`
	Enabled                   *bool    `json:"enabled,omitempty"`
	Passkey                   string   `json:"passkey,omitempty"`
	StatsMonitoring           *bool    `json:"stats_monitoring,omitempty"`
	AutoPrune                 *bool    `json:"auto_prune,omitempty"`
	SendUnreachableAlerts     *bool    `json:"send_unreachable_alerts,omitempty"`
	SendCPUAlerts             *bool    `json:"send_cpu_alerts,omitempty"`
	SendMemAlerts             *bool    `json:"send_mem_alerts,omitempty"`
	SendDiskAlerts            *bool    `json:"send_disk_alerts,omitempty"`
	SendVersionMismatchAlerts *bool    `json:"send_version_mismatch_alerts,omitempty"`
	CPUWarning                *float64 `json:"cpu_warning,omitempty"`
	CPUCritical               *float64 `json:"cpu_critical,omitempty"`
	MemWarning                *float64 `json:"mem_warning,omitempty"`
	MemCritical               *float64 `json:"mem_critical,omitempty"`
	DiskWarning               *float64 `json:"disk_warning,omitempty"`
	DiskCritical              *float64 `json:"disk_critical,omitempty"`
}

type Server struct {
	ID     string       `json:"-"`
	Name   string       `json:"name"`
	Tags   []string     `json:"tags,omitempty"`
	Config ServerConfig `json:"config"`
}

func (s *Server) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage `json:"_id"`
		Name   string          `json:"name"`
		Tags   []string        `json:"tags,omitempty"`
		Config ServerConfig    `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	s.ID = parseMongoID(a.ID)
	s.Name = a.Name
	s.Tags = a.Tags
	s.Config = a.Config
	return nil
}

type CreateServerParams struct {
	Name   string       `json:"name"`
	Config ServerConfig `json:"config,omitempty"`
}

type UpdateServerParams struct {
	ID     string       `json:"id"`
	Config ServerConfig `json:"config"`
}

type IDParam struct {
	ID string `json:"id"`
}

type NameOrIDParam struct {
	Server     string `json:"server,omitempty"`
	Stack      string `json:"stack,omitempty"`
	Deployment string `json:"deployment,omitempty"`
	Build      string `json:"build,omitempty"`
	Repo       string `json:"repo,omitempty"`
	Builder    string `json:"builder,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

func (c *Client) CreateServer(ctx context.Context, params CreateServerParams) (*Server, error) {
	var s Server
	err := c.Write(ctx, "CreateServer", params, &s)
	return &s, err
}

func (c *Client) GetServer(ctx context.Context, idOrName string) (*Server, error) {
	if idOrName == "" {
		return nil, &NotFoundError{Message: "server identifier is empty"}
	}
	var s Server
	err := c.Read(ctx, "GetServer", map[string]string{"server": idOrName}, &s)
	return &s, err
}

func (c *Client) UpdateServer(ctx context.Context, params UpdateServerParams) (*Server, error) {
	var s Server
	err := c.Write(ctx, "UpdateServer", params, &s)
	return &s, err
}

func (c *Client) DeleteServer(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteServer", IDParam{ID: id}, nil)
}

func (c *Client) ListServers(ctx context.Context) ([]Server, error) {
	var servers []Server
	err := c.Read(ctx, "ListServers", struct{}{}, &servers)
	return servers, err
}

// ---- Stack ----

// SystemCommand represents a shell command with an optional working directory.
type SystemCommand struct {
	Path    string `json:"path"`
	Command string `json:"command"`
}

type StackConfig struct {
	ServerID             string         `json:"server_id,omitempty"`
	ProjectName          string         `json:"project_name,omitempty"`
	AutoPull             *bool          `json:"auto_pull,omitempty"`
	RunBuild             *bool          `json:"run_build,omitempty"`
	AutoUpdate           *bool          `json:"auto_update,omitempty"`
	DestroyBeforeDeploy  *bool          `json:"destroy_before_deploy,omitempty"`
	GitProvider          string         `json:"git_provider,omitempty"`
	GitHTTPS             *bool          `json:"git_https,omitempty"`
	GitAccount           string         `json:"git_account,omitempty"`
	Repo                 string         `json:"repo,omitempty"`
	Branch               string         `json:"branch,omitempty"`
	Commit               string         `json:"commit,omitempty"`
	FilesOnHost          *bool          `json:"files_on_host,omitempty"`
	RunDirectory         string         `json:"run_directory,omitempty"`
	FilePaths            []string       `json:"file_paths,omitempty"`
	FileContents         string         `json:"file_contents,omitempty"`
	Environment          string         `json:"environment,omitempty"`
	EnvFilePath          string         `json:"env_file_path,omitempty"`
	AdditionalEnvFiles   []string       `json:"additional_env_files,omitempty"`
	WebhookEnabled       *bool          `json:"webhook_enabled,omitempty"`
	WebhookSecret        string         `json:"webhook_secret,omitempty"`
	WebhookForceDeploy   *bool          `json:"webhook_force_deploy,omitempty"`
	PreDeploy            *SystemCommand `json:"pre_deploy,omitempty"`
	PostDeploy           *SystemCommand `json:"post_deploy,omitempty"`
	ExtraArgs            []string       `json:"extra_args,omitempty"`
	BuildExtraArgs       []string       `json:"build_extra_args,omitempty"`
	IgnoreServices       []string       `json:"ignore_services,omitempty"`
	SendAlerts           *bool          `json:"send_alerts,omitempty"`
}

type Stack struct {
	ID     string      `json:"-"`
	Name   string      `json:"name"`
	Tags   []string    `json:"tags,omitempty"`
	Config StackConfig `json:"config"`
}

func (s *Stack) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage `json:"_id"`
		Name   string          `json:"name"`
		Tags   []string        `json:"tags,omitempty"`
		Config StackConfig     `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	s.ID = parseMongoID(a.ID)
	s.Name = a.Name
	s.Tags = a.Tags
	s.Config = a.Config
	return nil
}

type CreateStackParams struct {
	Name   string      `json:"name"`
	Config StackConfig `json:"config,omitempty"`
}

type UpdateStackParams struct {
	ID     string      `json:"id"`
	Config StackConfig `json:"config"`
}

func (c *Client) CreateStack(ctx context.Context, params CreateStackParams) (*Stack, error) {
	var s Stack
	err := c.Write(ctx, "CreateStack", params, &s)
	return &s, err
}

func (c *Client) GetStack(ctx context.Context, idOrName string) (*Stack, error) {
	if idOrName == "" {
		return nil, &NotFoundError{Message: "stack identifier is empty"}
	}
	var s Stack
	err := c.Read(ctx, "GetStack", map[string]string{"stack": idOrName}, &s)
	return &s, err
}

func (c *Client) UpdateStack(ctx context.Context, params UpdateStackParams) (*Stack, error) {
	var s Stack
	err := c.Write(ctx, "UpdateStack", params, &s)
	return &s, err
}

func (c *Client) DeleteStack(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteStack", IDParam{ID: id}, nil)
}

// StackService represents a single service within a stack and its running state.
type StackService struct {
	Service   string          `json:"service"`
	Image     string          `json:"image"`
	Container json.RawMessage `json:"container"` // null when not running
}

// IsRunning returns true when the service has an active container.
func (s StackService) IsRunning() bool {
	return len(s.Container) > 0 && string(s.Container) != "null"
}

func (c *Client) ListStackServices(ctx context.Context, stackID string) ([]StackService, error) {
	var services []StackService
	err := c.Read(ctx, "ListStackServices", map[string]string{"stack": stackID}, &services)
	return services, err
}

func (c *Client) StartStack(ctx context.Context, stackID string) error {
	return c.Execute(ctx, "StartStack", map[string]string{"stack": stackID}, nil)
}

func (c *Client) StopStack(ctx context.Context, stackID string) error {
	return c.Execute(ctx, "StopStack", map[string]string{"stack": stackID}, nil)
}

func (c *Client) DeployStack(ctx context.Context, stackID string) error {
	return c.Execute(ctx, "DeployStack", map[string]string{"stack": stackID}, nil)
}

func (c *Client) DestroyStack(ctx context.Context, stackID string) error {
	return c.Execute(ctx, "DestroyStack", map[string]interface{}{
		"stack":          stackID,
		"services":       []string{},
		"remove_orphans": false,
	}, nil)
}

// ---- Deployment ----

// DeploymentImage is an adjacently-tagged enum serialized as:
//
//	{"type": "Image", "params": {"image": "nginx:latest"}}
//	{"type": "Build", "params": {"build_id": "..."}}
type DeploymentImage struct {
	Type   string                `json:"type"`
	Params DeploymentImageParams `json:"params"`
}

type DeploymentImageParams struct {
	Image   string `json:"image,omitempty"`
	BuildID string `json:"build_id,omitempty"`
}

type DeploymentConfig struct {
	ServerID    string          `json:"server_id,omitempty"`
	Image       DeploymentImage `json:"image"`
	Network     string          `json:"network,omitempty"`
	RestartMode string          `json:"restart,omitempty"`
	Ports       string          `json:"ports,omitempty"`
	Volumes     string          `json:"volumes,omitempty"`
	Environment string          `json:"environment,omitempty"`
	Labels      string          `json:"labels,omitempty"`
	ExtraArgs   []string        `json:"extra_args,omitempty"`
	Command     string          `json:"command,omitempty"`
	SendAlerts  *bool           `json:"send_alerts,omitempty"`
	AutoUpdate  *bool           `json:"auto_update,omitempty"`
	Redeploy    *bool           `json:"redeploy_on_build,omitempty"`
}

type Deployment struct {
	ID     string           `json:"-"`
	Name   string           `json:"name"`
	Tags   []string         `json:"tags,omitempty"`
	Config DeploymentConfig `json:"config"`
}

func (d *Deployment) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage  `json:"_id"`
		Name   string           `json:"name"`
		Tags   []string         `json:"tags,omitempty"`
		Config DeploymentConfig `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	d.ID = parseMongoID(a.ID)
	d.Name = a.Name
	d.Tags = a.Tags
	d.Config = a.Config
	return nil
}

type CreateDeploymentParams struct {
	Name   string           `json:"name"`
	Config DeploymentConfig `json:"config,omitempty"`
}

type UpdateDeploymentParams struct {
	ID     string           `json:"id"`
	Config DeploymentConfig `json:"config"`
}

func (c *Client) CreateDeployment(ctx context.Context, params CreateDeploymentParams) (*Deployment, error) {
	var d Deployment
	err := c.Write(ctx, "CreateDeployment", params, &d)
	return &d, err
}

func (c *Client) GetDeployment(ctx context.Context, idOrName string) (*Deployment, error) {
	var d Deployment
	err := c.Read(ctx, "GetDeployment", map[string]string{"deployment": idOrName}, &d)
	return &d, err
}

func (c *Client) UpdateDeployment(ctx context.Context, params UpdateDeploymentParams) (*Deployment, error) {
	var d Deployment
	err := c.Write(ctx, "UpdateDeployment", params, &d)
	return &d, err
}

func (c *Client) DeleteDeployment(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteDeployment", IDParam{ID: id}, nil)
}

func (c *Client) DeployDeployment(ctx context.Context, idOrName string) error {
	return c.Execute(ctx, "Deploy", map[string]string{"deployment": idOrName}, nil)
}

func (c *Client) DestroyDeployment(ctx context.Context, idOrName string) error {
	return c.Execute(ctx, "DestroyDeployment", map[string]string{"deployment": idOrName}, nil)
}

// ---- Build ----

type BuildConfig struct {
	BuilderID            string   `json:"builder_id,omitempty"`
	ImageName            string   `json:"image_name,omitempty"`
	ImageTag             string   `json:"image_tag,omitempty"`
	AutoIncrementVersion *bool    `json:"auto_increment_version,omitempty"`
	GitProvider          string   `json:"git_provider,omitempty"`
	GitHTTPS             *bool    `json:"git_https,omitempty"`
	GitAccount           string   `json:"git_account,omitempty"`
	Repo                 string   `json:"repo,omitempty"`
	Branch               string   `json:"branch,omitempty"`
	Commit               string   `json:"commit,omitempty"`
	BuildPath            string   `json:"build_path,omitempty"`
	DockerfilePath       string   `json:"dockerfile_path,omitempty"`
	UseBuildx            *bool    `json:"use_buildx,omitempty"`
	ExtraArgs            []string `json:"extra_args,omitempty"`
	BuildArgs            string   `json:"build_args,omitempty"`
	SecretArgs           string   `json:"secret_args,omitempty"`
	Labels               string   `json:"labels,omitempty"`
	WebhookEnabled       *bool    `json:"webhook_enabled,omitempty"`
	FilesOnHost          *bool    `json:"files_on_host,omitempty"`
}

type Build struct {
	ID     string      `json:"-"`
	Name   string      `json:"name"`
	Tags   []string    `json:"tags,omitempty"`
	Config BuildConfig `json:"config"`
}

func (b *Build) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage `json:"_id"`
		Name   string          `json:"name"`
		Tags   []string        `json:"tags,omitempty"`
		Config BuildConfig     `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	b.ID = parseMongoID(a.ID)
	b.Name = a.Name
	b.Tags = a.Tags
	b.Config = a.Config
	return nil
}

type CreateBuildParams struct {
	Name   string      `json:"name"`
	Config BuildConfig `json:"config,omitempty"`
}

type UpdateBuildParams struct {
	ID     string      `json:"id"`
	Config BuildConfig `json:"config"`
}

func (c *Client) CreateBuild(ctx context.Context, params CreateBuildParams) (*Build, error) {
	var b Build
	err := c.Write(ctx, "CreateBuild", params, &b)
	return &b, err
}

func (c *Client) GetBuild(ctx context.Context, idOrName string) (*Build, error) {
	var b Build
	err := c.Read(ctx, "GetBuild", map[string]string{"build": idOrName}, &b)
	return &b, err
}

func (c *Client) UpdateBuild(ctx context.Context, params UpdateBuildParams) (*Build, error) {
	var b Build
	err := c.Write(ctx, "UpdateBuild", params, &b)
	return &b, err
}

func (c *Client) DeleteBuild(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteBuild", IDParam{ID: id}, nil)
}

// ---- Repo ----

type RepoConfig struct {
	ServerID       string `json:"server_id,omitempty"`
	GitProvider    string `json:"git_provider,omitempty"`
	GitHTTPS       *bool  `json:"git_https,omitempty"`
	GitAccount     string `json:"git_account,omitempty"`
	Repo           string `json:"repo,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Commit         string `json:"commit,omitempty"`
	OnClone        string `json:"on_clone,omitempty"`
	OnPull         string `json:"on_pull,omitempty"`
	WebhookEnabled *bool  `json:"webhook_enabled,omitempty"`
}

type Repo struct {
	ID     string     `json:"-"`
	Name   string     `json:"name"`
	Tags   []string   `json:"tags,omitempty"`
	Config RepoConfig `json:"config"`
}

func (r *Repo) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage `json:"_id"`
		Name   string          `json:"name"`
		Tags   []string        `json:"tags,omitempty"`
		Config RepoConfig      `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	r.ID = parseMongoID(a.ID)
	r.Name = a.Name
	r.Tags = a.Tags
	r.Config = a.Config
	return nil
}

type CreateRepoParams struct {
	Name   string     `json:"name"`
	Config RepoConfig `json:"config,omitempty"`
}

type UpdateRepoParams struct {
	ID     string     `json:"id"`
	Config RepoConfig `json:"config"`
}

func (c *Client) CreateRepo(ctx context.Context, params CreateRepoParams) (*Repo, error) {
	var r Repo
	err := c.Write(ctx, "CreateRepo", params, &r)
	return &r, err
}

func (c *Client) GetRepo(ctx context.Context, idOrName string) (*Repo, error) {
	var r Repo
	err := c.Read(ctx, "GetRepo", map[string]string{"repo": idOrName}, &r)
	return &r, err
}

func (c *Client) UpdateRepo(ctx context.Context, params UpdateRepoParams) (*Repo, error) {
	var r Repo
	err := c.Write(ctx, "UpdateRepo", params, &r)
	return &r, err
}

func (c *Client) DeleteRepo(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteRepo", IDParam{ID: id}, nil)
}

// ---- Tag ----

type Tag struct {
	ID    string `json:"-"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

func (t *Tag) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID    json.RawMessage `json:"_id"`
		Name  string          `json:"name"`
		Color string          `json:"color,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	t.ID = parseMongoID(a.ID)
	t.Name = a.Name
	t.Color = a.Color
	return nil
}

type CreateTagParams struct {
	Name string `json:"name"`
}

func (c *Client) CreateTag(ctx context.Context, params CreateTagParams) (*Tag, error) {
	var t Tag
	err := c.Write(ctx, "CreateTag", params, &t)
	return &t, err
}

func (c *Client) GetTag(ctx context.Context, idOrName string) (*Tag, error) {
	// Tags are read via ListTags — filter client-side.
	tags, err := c.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.ID == idOrName || t.Name == idOrName {
			return &t, nil
		}
	}
	return nil, &NotFoundError{Message: fmt.Sprintf("tag not found: %s", idOrName)}
}

func (c *Client) ListTags(ctx context.Context) ([]Tag, error) {
	var tags []Tag
	err := c.Read(ctx, "ListTags", struct{}{}, &tags)
	return tags, err
}

func (c *Client) RenameTag(ctx context.Context, id string, name string) (*Tag, error) {
	var t Tag
	err := c.Write(ctx, "RenameTag", map[string]string{"id": id, "name": name}, &t)
	return &t, err
}

func (c *Client) UpdateTagColor(ctx context.Context, id string, color string) (*Tag, error) {
	var t Tag
	err := c.Write(ctx, "UpdateTagColor", map[string]string{"id": id, "color": color}, &t)
	return &t, err
}

func (c *Client) DeleteTag(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteTag", IDParam{ID: id}, nil)
}

// ---- ResourceMeta ----

type ResourceTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type UpdateResourceMetaParams struct {
	Target ResourceTarget `json:"target"`
	Tags   []string       `json:"tags"`
}

func (c *Client) UpdateResourceMeta(ctx context.Context, targetType, id string, tags []string) error {
	return c.Write(ctx, "UpdateResourceMeta", UpdateResourceMetaParams{
		Target: ResourceTarget{Type: targetType, ID: id},
		Tags:   tags,
	}, nil)
}

// ---- Builder ----

type BuilderConfig struct {
	ServerID string `json:"server_id,omitempty"`
}

type Builder struct {
	ID     string        `json:"-"`
	Name   string        `json:"name"`
	Tags   []string      `json:"tags,omitempty"`
	Config BuilderConfig `json:"config"`
}

func (b *Builder) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID     json.RawMessage `json:"_id"`
		Name   string          `json:"name"`
		Tags   []string        `json:"tags,omitempty"`
		Config BuilderConfig   `json:"config"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	b.ID = parseMongoID(a.ID)
	b.Name = a.Name
	b.Tags = a.Tags
	b.Config = a.Config
	return nil
}

type CreateBuilderParams struct {
	Name   string        `json:"name"`
	Config BuilderConfig `json:"config,omitempty"`
}

type UpdateBuilderParams struct {
	ID     string        `json:"id"`
	Config BuilderConfig `json:"config"`
}

func (c *Client) CreateBuilder(ctx context.Context, params CreateBuilderParams) (*Builder, error) {
	var b Builder
	err := c.Write(ctx, "CreateBuilder", params, &b)
	return &b, err
}

func (c *Client) GetBuilder(ctx context.Context, idOrName string) (*Builder, error) {
	var b Builder
	err := c.Read(ctx, "GetBuilder", map[string]string{"builder": idOrName}, &b)
	return &b, err
}

func (c *Client) UpdateBuilder(ctx context.Context, params UpdateBuilderParams) (*Builder, error) {
	var b Builder
	err := c.Write(ctx, "UpdateBuilder", params, &b)
	return &b, err
}

func (c *Client) DeleteBuilder(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteBuilder", IDParam{ID: id}, nil)
}

// ---- User ----

type User struct {
	ID                      string `json:"-"`
	Username                string `json:"username"`
	Enabled                 bool   `json:"enabled"`
	Admin                   bool   `json:"admin"`
	SuperAdmin              bool   `json:"super_admin"`
	CreateServerPermissions bool   `json:"create_server_permissions"`
	CreateBuildPermissions  bool   `json:"create_build_permissions"`
}

func (u *User) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID                      json.RawMessage `json:"_id"`
		Username                string          `json:"username"`
		Enabled                 bool            `json:"enabled"`
		Admin                   bool            `json:"admin"`
		SuperAdmin              bool            `json:"super_admin"`
		CreateServerPermissions bool            `json:"create_server_permissions"`
		CreateBuildPermissions  bool            `json:"create_build_permissions"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	u.ID = parseMongoID(a.ID)
	u.Username = a.Username
	u.Enabled = a.Enabled
	u.Admin = a.Admin
	u.SuperAdmin = a.SuperAdmin
	u.CreateServerPermissions = a.CreateServerPermissions
	u.CreateBuildPermissions = a.CreateBuildPermissions
	return nil
}

func (c *Client) CreateLocalUser(ctx context.Context, username, password string) (*User, error) {
	var u User
	err := c.Write(ctx, "CreateLocalUser", map[string]string{
		"username": username,
		"password": password,
	}, &u)
	return &u, err
}

func (c *Client) CreateServiceUser(ctx context.Context, username, description string) (*User, error) {
	var u User
	err := c.Write(ctx, "CreateServiceUser", map[string]string{
		"username":    username,
		"description": description,
	}, &u)
	return &u, err
}

func (c *Client) FindUser(ctx context.Context, idOrName string) (*User, error) {
	var u User
	err := c.Read(ctx, "FindUser", map[string]string{"user": idOrName}, &u)
	return &u, err
}

func (c *Client) UpdateUserBasePermissions(ctx context.Context, userID string, enabled, createServer, createBuild bool) error {
	return c.Write(ctx, "UpdateUserBasePermissions", map[string]interface{}{
		"user_id":                   userID,
		"enabled":                   enabled,
		"create_server_permissions": createServer,
		"create_build_permissions":  createBuild,
	}, nil)
}

func (c *Client) UpdateUserAdmin(ctx context.Context, userID string, admin bool) error {
	return c.Write(ctx, "UpdateUserAdmin", map[string]interface{}{
		"user_id": userID,
		"admin":   admin,
	}, nil)
}

func (c *Client) DeleteUser(ctx context.Context, userID string) error {
	return c.Write(ctx, "DeleteUser", map[string]string{"user": userID}, nil)
}

// ---- API Keys ----

type ApiKeyResponse struct {
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

func (c *Client) CreateApiKeyForServiceUser(ctx context.Context, userID, name string) (*ApiKeyResponse, error) {
	var r ApiKeyResponse
	err := c.Write(ctx, "CreateApiKeyForServiceUser", map[string]string{
		"user_id": userID,
		"name":    name,
	}, &r)
	return &r, err
}

func (c *Client) DeleteApiKeyForServiceUser(ctx context.Context, key string) error {
	return c.Write(ctx, "DeleteApiKeyForServiceUser", map[string]string{"key": key}, nil)
}

// ---- Git Provider Accounts ----

type GitProviderAccount struct {
	ID       string `json:"id"`
	Domain   string `json:"domain"`
	Https    bool   `json:"https"`
	Username string `json:"username"`
	Token    string `json:"token"`
}

func (g *GitProviderAccount) UnmarshalJSON(data []byte) error {
	type alias struct {
		ID       json.RawMessage `json:"_id"`
		Domain   string          `json:"domain"`
		Https    bool            `json:"https"`
		Username string          `json:"username"`
		Token    string          `json:"token"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	g.ID = parseMongoID(a.ID)
	g.Domain = a.Domain
	g.Https = a.Https
	g.Username = a.Username
	g.Token = a.Token
	return nil
}

func (c *Client) CreateGitProviderAccount(ctx context.Context, domain, username, token string, https bool) (*GitProviderAccount, error) {
	var g GitProviderAccount
	err := c.Write(ctx, "CreateGitProviderAccount", map[string]interface{}{
		"account": map[string]interface{}{
			"domain":   domain,
			"username": username,
			"token":    token,
			"https":    https,
		},
	}, &g)
	return &g, err
}

func (c *Client) UpdateGitProviderAccount(ctx context.Context, id, domain, username, token string, https bool) (*GitProviderAccount, error) {
	var g GitProviderAccount
	err := c.Write(ctx, "UpdateGitProviderAccount", map[string]interface{}{
		"id": id,
		"account": map[string]interface{}{
			"domain":   domain,
			"username": username,
			"token":    token,
			"https":    https,
		},
	}, &g)
	return &g, err
}

func (c *Client) GetGitProviderAccount(ctx context.Context, id string) (*GitProviderAccount, error) {
	var g GitProviderAccount
	err := c.Read(ctx, "GetGitProviderAccount", map[string]string{"id": id}, &g)
	if err != nil {
		return nil, err
	}
	if g.ID == "" {
		return nil, &NotFoundError{Message: fmt.Sprintf("git provider account %q not found", id)}
	}
	return &g, nil
}

func (c *Client) DeleteGitProviderAccount(ctx context.Context, id string) error {
	return c.Write(ctx, "DeleteGitProviderAccount", map[string]string{"id": id}, nil)
}

// Helper to get a bool pointer
func BoolPtr(b bool) *bool {
	return &b
}

func Float64Ptr(f float64) *float64 {
	return &f
}
