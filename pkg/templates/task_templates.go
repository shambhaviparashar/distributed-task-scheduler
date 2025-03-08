package templates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// ParameterType represents the type of a template parameter
type ParameterType string

const (
	// TypeString represents a string parameter
	TypeString ParameterType = "string"
	// TypeInteger represents an integer parameter
	TypeInteger ParameterType = "integer"
	// TypeFloat represents a float parameter
	TypeFloat ParameterType = "float"
	// TypeBoolean represents a boolean parameter
	TypeBoolean ParameterType = "boolean"
	// TypeArray represents an array parameter
	TypeArray ParameterType = "array"
	// TypeObject represents an object parameter
	TypeObject ParameterType = "object"
	// TypeDateTime represents a date-time parameter
	TypeDateTime ParameterType = "datetime"
)

// ParameterDefinition defines a parameter for a task template
type ParameterDefinition struct {
	Name        string        `json:"name"`
	Type        ParameterType `json:"type"`
	Description string        `json:"description"`
	Required    bool          `json:"required"`
	Default     interface{}   `json:"default,omitempty"`
	Example     interface{}   `json:"example,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`      // Allowed values
	Pattern     string        `json:"pattern,omitempty"`   // Regex pattern for validation (for strings)
	MinValue    *float64      `json:"min_value,omitempty"` // Minimum value (for numeric types)
	MaxValue    *float64      `json:"max_value,omitempty"` // Maximum value (for numeric types)
	MinLength   *int          `json:"min_length,omitempty"` // Minimum length (for strings and arrays)
	MaxLength   *int          `json:"max_length,omitempty"` // Maximum length (for strings and arrays)
}

// ScheduleConfig defines when a task should be scheduled
type ScheduleConfig struct {
	Type         string `json:"type"`          // "once", "recurring", "cron"
	Interval     string `json:"interval,omitempty"`      // For recurring tasks, e.g., "30s", "5m", "1h"
	CronSchedule string `json:"cron_schedule,omitempty"` // For cron tasks
	StartTime    string `json:"start_time,omitempty"`    // ISO8601 time for first execution
	EndTime      string `json:"end_time,omitempty"`      // ISO8601 time for last execution
	Timezone     string `json:"timezone,omitempty"`      // Timezone for schedule evaluation
}

// ResourceConfig defines resource requirements for a task
type ResourceConfig struct {
	CPU         string `json:"cpu,omitempty"`         // CPU requirements, e.g., "0.5", "1"
	Memory      string `json:"memory,omitempty"`      // Memory requirements, e.g., "512Mi", "1Gi"
	DiskStorage string `json:"disk_storage,omitempty"` // Disk requirements, e.g., "1Gi"
	GPU         int    `json:"gpu,omitempty"`         // Number of GPUs
	Preemptible bool   `json:"preemptible"`           // Whether the task can be preempted
}

// RetryConfig defines retry behavior for a task
type RetryConfig struct {
	MaxRetries     int     `json:"max_retries"`
	InitialBackoff string  `json:"initial_backoff"`         // Duration string, e.g., "30s"
	MaxBackoff     string  `json:"max_backoff,omitempty"`   // Duration string, e.g., "1h"
	BackoffFactor  float64 `json:"backoff_factor"`          // Multiplier for backoff
	RetryOnErrors  []string `json:"retry_on_errors,omitempty"` // Specific errors to retry on
}

// NotificationConfig defines notification settings for a task
type NotificationConfig struct {
	OnSuccess bool     `json:"on_success"`
	OnFailure bool     `json:"on_failure"`
	OnRetry   bool     `json:"on_retry"`
	Channels  []string `json:"channels,omitempty"` // Notification channels: "email", "slack", etc.
	Recipients []string `json:"recipients,omitempty"` // List of recipients
}

// TaskTemplate defines a reusable template for creating tasks
type TaskTemplate struct {
	ID           string                        `json:"id"`
	Name         string                        `json:"name"`
	Description  string                        `json:"description"`
	Version      string                        `json:"version"`
	Command      string                        `json:"command"` // Command template with parameter placeholders
	ShellType    string                        `json:"shell_type,omitempty"` // e.g., "bash", "powershell"
	WorkDir      string                        `json:"work_dir,omitempty"`
	Parameters   map[string]ParameterDefinition `json:"parameters"`
	Environment  map[string]string             `json:"environment,omitempty"` // Environment variables
	Schedule     *ScheduleConfig               `json:"schedule,omitempty"`
	Resources    ResourceConfig                `json:"resources"`
	RetryPolicy  RetryConfig                   `json:"retry_policy"`
	Timeout      string                        `json:"timeout"`       // Duration string, e.g., "1h"
	Tags         []string                      `json:"tags,omitempty"`
	Priority     int                           `json:"priority"`      // 1-10, higher is more important
	Notifications *NotificationConfig          `json:"notifications,omitempty"`
	CreatedAt    time.Time                     `json:"created_at"`
	UpdatedAt    time.Time                     `json:"updated_at"`
}

// TaskParameters contains parameter values for task instantiation
type TaskParameters map[string]interface{}

// Validate checks if the task template is valid
func (tt *TaskTemplate) Validate() error {
	if tt.ID == "" {
		return errors.New("template ID cannot be empty")
	}
	if tt.Name == "" {
		return errors.New("template name cannot be empty")
	}
	if tt.Command == "" {
		return errors.New("template command cannot be empty")
	}

	// Validate parameter definitions
	for name, param := range tt.Parameters {
		if param.Name != name {
			return fmt.Errorf("parameter name mismatch: %s vs %s", name, param.Name)
		}

		// Validate parameter types and default values
		if param.Default != nil {
			if err := validateParameterType(param.Type, param.Default); err != nil {
				return fmt.Errorf("invalid default value for parameter %s: %v", name, err)
			}
		}

		// Validate enum values
		if len(param.Enum) > 0 {
			for _, enumValue := range param.Enum {
				if err := validateParameterType(param.Type, enumValue); err != nil {
					return fmt.Errorf("invalid enum value for parameter %s: %v", name, err)
				}
			}
		}

		// Validate pattern
		if param.Pattern != "" {
			_, err := regexp.Compile(param.Pattern)
			if err != nil {
				return fmt.Errorf("invalid pattern for parameter %s: %v", name, err)
			}
		}
	}

	// Validate command template
	_, err := template.New("command").Parse(tt.Command)
	if err != nil {
		return fmt.Errorf("invalid command template: %v", err)
	}

	// Validate schedule configuration
	if tt.Schedule != nil {
		if err := validateSchedule(tt.Schedule); err != nil {
			return fmt.Errorf("invalid schedule configuration: %v", err)
		}
	}

	return nil
}

// validateParameterType checks if a value matches the expected type
func validateParameterType(paramType ParameterType, value interface{}) error {
	switch paramType {
	case TypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case TypeInteger:
		switch v := value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// These types are all fine
		case float64:
			if v != float64(int(v)) {
				return fmt.Errorf("expected integer, got float %v", v)
			}
		default:
			return fmt.Errorf("expected integer, got %T", value)
		}
	case TypeFloat:
		switch value.(type) {
		case float32, float64:
			// These types are fine
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			// Integer types can be converted to float
		default:
			return fmt.Errorf("expected float, got %T", value)
		}
	case TypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case TypeArray:
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	case TypeObject:
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	case TypeDateTime:
		switch v := value.(type) {
		case string:
			_, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return fmt.Errorf("expected RFC3339 datetime string, got %v", v)
			}
		case time.Time:
			// This type is fine
		default:
			return fmt.Errorf("expected datetime, got %T", value)
		}
	default:
		return fmt.Errorf("unknown parameter type: %s", paramType)
	}
	return nil
}

// validateSchedule checks if a schedule configuration is valid
func validateSchedule(schedule *ScheduleConfig) error {
	switch schedule.Type {
	case "once":
		if schedule.StartTime == "" {
			return errors.New("start time is required for 'once' schedule")
		}
		_, err := time.Parse(time.RFC3339, schedule.StartTime)
		if err != nil {
			return fmt.Errorf("invalid start time: %v", err)
		}
	case "recurring":
		if schedule.Interval == "" {
			return errors.New("interval is required for 'recurring' schedule")
		}
		// TODO: Add validation for duration string
	case "cron":
		if schedule.CronSchedule == "" {
			return errors.New("cron expression is required for 'cron' schedule")
		}
		// TODO: Add cron expression validation
	default:
		return fmt.Errorf("unknown schedule type: %s", schedule.Type)
	}
	return nil
}

// ValidateParameterValues validates that provided parameters match the template's definitions
func (tt *TaskTemplate) ValidateParameterValues(params TaskParameters) error {
	// Check for required parameters
	for name, paramDef := range tt.Parameters {
		value, exists := params[name]
		
		if paramDef.Required && !exists {
			// Check if default is available
			if paramDef.Default != nil {
				// Use default value
				params[name] = paramDef.Default
			} else {
				return fmt.Errorf("required parameter %s is missing", name)
			}
		}
		
		// Skip validation for non-provided optional parameters
		if !exists {
			continue
		}
		
		// Validate parameter type
		if err := validateParameterType(paramDef.Type, value); err != nil {
			return fmt.Errorf("parameter %s: %v", name, err)
		}
		
		// Validate string pattern
		if paramDef.Pattern != "" && paramDef.Type == TypeString {
			strValue, _ := value.(string)
			regex, _ := regexp.Compile(paramDef.Pattern)
			if !regex.MatchString(strValue) {
				return fmt.Errorf("parameter %s does not match pattern %s", name, paramDef.Pattern)
			}
		}
		
		// Validate numeric ranges
		if (paramDef.Type == TypeInteger || paramDef.Type == TypeFloat) && 
		   (paramDef.MinValue != nil || paramDef.MaxValue != nil) {
			var numValue float64
			switch v := value.(type) {
			case int:
				numValue = float64(v)
			case int64:
				numValue = float64(v)
			case float64:
				numValue = v
			}
			
			if paramDef.MinValue != nil && numValue < *paramDef.MinValue {
				return fmt.Errorf("parameter %s is below minimum value %v", name, *paramDef.MinValue)
			}
			if paramDef.MaxValue != nil && numValue > *paramDef.MaxValue {
				return fmt.Errorf("parameter %s is above maximum value %v", name, *paramDef.MaxValue)
			}
		}
		
		// Validate string/array length
		if paramDef.Type == TypeString {
			strValue, _ := value.(string)
			if paramDef.MinLength != nil && len(strValue) < *paramDef.MinLength {
				return fmt.Errorf("parameter %s is too short (min length: %d)", name, *paramDef.MinLength)
			}
			if paramDef.MaxLength != nil && len(strValue) > *paramDef.MaxLength {
				return fmt.Errorf("parameter %s is too long (max length: %d)", name, *paramDef.MaxLength)
			}
		} else if paramDef.Type == TypeArray {
			arrValue, _ := value.([]interface{})
			if paramDef.MinLength != nil && len(arrValue) < *paramDef.MinLength {
				return fmt.Errorf("parameter %s has too few items (min: %d)", name, *paramDef.MinLength)
			}
			if paramDef.MaxLength != nil && len(arrValue) > *paramDef.MaxLength {
				return fmt.Errorf("parameter %s has too many items (max: %d)", name, *paramDef.MaxLength)
			}
		}
		
		// Validate enum values
		if len(paramDef.Enum) > 0 {
			found := false
			for _, enumValue := range paramDef.Enum {
				if value == enumValue {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("parameter %s value not in allowed values: %v", name, paramDef.Enum)
			}
		}
	}
	
	// Check for unknown parameters
	for name := range params {
		if _, exists := tt.Parameters[name]; !exists {
			return fmt.Errorf("unknown parameter: %s", name)
		}
	}
	
	return nil
}

// InstantiateCommand creates a command string with parameters applied
func (tt *TaskTemplate) InstantiateCommand(params TaskParameters) (string, error) {
	// Validate parameters first
	if err := tt.ValidateParameterValues(params); err != nil {
		return "", err
	}
	
	// Create a template
	tmpl, err := template.New("command").Parse(tt.Command)
	if err != nil {
		return "", fmt.Errorf("failed to parse command template: %v", err)
	}
	
	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute command template: %v", err)
	}
	
	return buf.String(), nil
}

// InstantiateEnvironment creates environment variables with parameters applied
func (tt *TaskTemplate) InstantiateEnvironment(params TaskParameters) (map[string]string, error) {
	// Validate parameters first
	if err := tt.ValidateParameterValues(params); err != nil {
		return nil, err
	}
	
	// If no environment variables, return empty map
	if len(tt.Environment) == 0 {
		return make(map[string]string), nil
	}
	
	// Apply parameters to environment variables
	env := make(map[string]string)
	for key, value := range tt.Environment {
		// Create a template for each environment variable
		tmpl, err := template.New(key).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse environment variable template %s: %v", key, err)
		}
		
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, params); err != nil {
			return nil, fmt.Errorf("failed to execute environment variable template %s: %v", key, err)
		}
		
		env[key] = buf.String()
	}
	
	return env, nil
}

// CreateTaskFromTemplate creates a task definition from the template with provided parameters
func CreateTaskFromTemplate(tt *TaskTemplate, taskID string, params TaskParameters) (map[string]interface{}, error) {
	// Validate and instantiate command
	command, err := tt.InstantiateCommand(params)
	if err != nil {
		return nil, err
	}
	
	// Instantiate environment variables
	env, err := tt.InstantiateEnvironment(params)
	if err != nil {
		return nil, err
	}
	
	// Create task definition
	task := map[string]interface{}{
		"id":           taskID,
		"template_id":  tt.ID,
		"name":         tt.Name,
		"description":  tt.Description,
		"command":      command,
		"environment":  env,
		"work_dir":     tt.WorkDir,
		"priority":     tt.Priority,
		"resources":    tt.Resources,
		"retry_policy": tt.RetryPolicy,
		"timeout":      tt.Timeout,
		"parameters":   params,
		"tags":         tt.Tags,
		"created_at":   time.Now(),
	}
	
	// Add schedule if defined
	if tt.Schedule != nil {
		task["schedule"] = tt.Schedule
	}
	
	// Add notifications if defined
	if tt.Notifications != nil {
		task["notifications"] = tt.Notifications
	}
	
	return task, nil
}

// LoadTemplateFromJSON loads a task template from JSON
func LoadTemplateFromJSON(data []byte) (*TaskTemplate, error) {
	var template TaskTemplate
	err := json.Unmarshal(data, &template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template JSON: %v", err)
	}
	
	// Validate the template
	if err := template.Validate(); err != nil {
		return nil, err
	}
	
	return &template, nil
}

// ToJSON converts a task template to JSON
func (tt *TaskTemplate) ToJSON() ([]byte, error) {
	return json.MarshalIndent(tt, "", "  ")
}

// GetParameterInfo returns user-friendly information about the template parameters
func (tt *TaskTemplate) GetParameterInfo() string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("Template: %s (ID: %s, Version: %s)\n", tt.Name, tt.ID, tt.Version))
	builder.WriteString("Parameters:\n")
	
	for name, param := range tt.Parameters {
		required := ""
		if param.Required {
			required = " (required)"
		}
		
		defaultValue := ""
		if param.Default != nil {
			defaultValue = fmt.Sprintf(", default: %v", param.Default)
		}
		
		builder.WriteString(fmt.Sprintf("  - %s: %s%s%s\n", name, param.Type, required, defaultValue))
		
		if param.Description != "" {
			builder.WriteString(fmt.Sprintf("    Description: %s\n", param.Description))
		}
		
		if len(param.Enum) > 0 {
			builder.WriteString(fmt.Sprintf("    Allowed values: %v\n", param.Enum))
		}
		
		if param.Pattern != "" {
			builder.WriteString(fmt.Sprintf("    Pattern: %s\n", param.Pattern))
		}
		
		if param.MinValue != nil || param.MaxValue != nil {
			range_ := "Range: "
			if param.MinValue != nil {
				range_ += fmt.Sprintf("%v ≤ ", *param.MinValue)
			}
			range_ += "value"
			if param.MaxValue != nil {
				range_ += fmt.Sprintf(" ≤ %v", *param.MaxValue)
			}
			builder.WriteString(fmt.Sprintf("    %s\n", range_))
		}
		
		if param.MinLength != nil || param.MaxLength != nil {
			length := "Length: "
			if param.MinLength != nil {
				length += fmt.Sprintf("%d ≤ ", *param.MinLength)
			}
			length += "length"
			if param.MaxLength != nil {
				length += fmt.Sprintf(" ≤ %d", *param.MaxLength)
			}
			builder.WriteString(fmt.Sprintf("    %s\n", length))
		}
	}
	
	return builder.String()
}

// GetCommandPreview returns a preview of the command with example parameter values
func (tt *TaskTemplate) GetCommandPreview() (string, error) {
	// Create a map of example parameter values
	params := make(TaskParameters)
	
	for name, param := range tt.Parameters {
		if param.Example != nil {
			params[name] = param.Example
		} else if param.Default != nil {
			params[name] = param.Default
		} else {
			// Use a placeholder value based on the type
			switch param.Type {
			case TypeString:
				params[name] = fmt.Sprintf("<%s>", name)
			case TypeInteger:
				params[name] = 0
			case TypeFloat:
				params[name] = 0.0
			case TypeBoolean:
				params[name] = false
			case TypeArray:
				params[name] = []interface{}{}
			case TypeObject:
				params[name] = map[string]interface{}{}
			case TypeDateTime:
				params[name] = time.Now().Format(time.RFC3339)
			}
		}
	}
	
	// Instantiate command (skip validation)
	tmpl, err := template.New("command").Parse(tt.Command)
	if err != nil {
		return "", fmt.Errorf("failed to parse command template: %v", err)
	}
	
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute command template: %v", err)
	}
	
	return buf.String(), nil
}

// Helper functions for common parameter types

// NewStringParameter creates a new string parameter definition
func NewStringParameter(name, description string, required bool, defaultValue string) ParameterDefinition {
	return ParameterDefinition{
		Name:        name,
		Type:        TypeString,
		Description: description,
		Required:    required,
		Default:     defaultValue,
	}
}

// NewIntegerParameter creates a new integer parameter definition
func NewIntegerParameter(name, description string, required bool, defaultValue int) ParameterDefinition {
	return ParameterDefinition{
		Name:        name,
		Type:        TypeInteger,
		Description: description,
		Required:    required,
		Default:     defaultValue,
	}
}

// NewBooleanParameter creates a new boolean parameter definition
func NewBooleanParameter(name, description string, required bool, defaultValue bool) ParameterDefinition {
	return ParameterDefinition{
		Name:        name,
		Type:        TypeBoolean,
		Description: description,
		Required:    required,
		Default:     defaultValue,
	}
}

// ParseDuration parses a duration string with support for days and weeks
func ParseDuration(duration string) (time.Duration, error) {
	// Handle days (e.g., "3d")
	if strings.HasSuffix(duration, "d") {
		daysStr := strings.TrimSuffix(duration, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, fmt.Errorf("invalid day format: %s", duration)
		}
		return time.Hour * 24 * time.Duration(days), nil
	}
	
	// Handle weeks (e.g., "2w")
	if strings.HasSuffix(duration, "w") {
		weeksStr := strings.TrimSuffix(duration, "w")
		weeks, err := strconv.Atoi(weeksStr)
		if err != nil {
			return 0, fmt.Errorf("invalid week format: %s", duration)
		}
		return time.Hour * 24 * 7 * time.Duration(weeks), nil
	}
	
	// For standard durations (e.g., "30s", "5m", "2h")
	return time.ParseDuration(duration)
} 