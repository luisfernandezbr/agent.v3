package api

import "time"

type UserModel struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

type DiscussionModel struct {
	ID    string `json:"id"`
	Notes []struct {
		ID        int       `json:"id"`
		Author    UserModel `json:"author"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		System    bool      `json:"system"`
	} `json:"notes"`
}

type IssueModel struct {
	ID          int       `json:"id"`
	Iid         int       `json:"iid"`
	ProjectID   int       `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Labels      []string  `json:"labels"`
	Milestone   struct {
		ID          int       `json:"id"`
		Iid         int       `json:"iid"`
		GroupID     int       `json:"group_id"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		State       string    `json:"state"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
		DueDate     string    `json:"due_date"`
		StartDate   string    `json:"start_date"`
		WebURL      string    `json:"web_url"`
	} `json:"milestone"`
	Assignees          []UserModel `json:"assignees"`
	Author             UserModel   `json:"author"`
	Assignee           UserModel   `json:"assignee"`
	UserNotesCount     int         `json:"user_notes_count"`
	MergeRequestsCount int         `json:"merge_requests_count"`
	Upvotes            int         `json:"upvotes"`
	Downvotes          int         `json:"downvotes"`
	DueDate            string      `json:"due_date"`
	Confidential       bool        `json:"confidential"`
	DiscussionLocked   interface{} `json:"discussion_locked"`
	WebURL             string      `json:"web_url"`
	TimeStats          struct {
		TimeEstimate        int         `json:"time_estimate"`
		TotalTimeSpent      int         `json:"total_time_spent"`
		HumanTimeEstimate   interface{} `json:"human_time_estimate"`
		HumanTotalTimeSpent interface{} `json:"human_total_time_spent"`
	} `json:"time_stats"`
	TaskCompletionStatus struct {
		Count          int `json:"count"`
		CompletedCount int `json:"completed_count"`
	} `json:"task_completion_status"`
	Weight   interface{} `json:"weight"`
	HasTasks bool        `json:"has_tasks"`
	Links    struct {
		Self       string `json:"self"`
		Notes      string `json:"notes"`
		AwardEmoji string `json:"award_emoji"`
		Project    string `json:"project"`
	} `json:"_links"`
	References struct {
		Short    string `json:"short"`
		Relative string `json:"relative"`
		Full     string `json:"full"`
	} `json:"references"`
	MovedToID interface{} `json:"moved_to_id"`
	EpicIid   int         `json:"epic_iid"`
	Epic      interface{} `json:"epic"`
}

type IssueMilestone struct {
	ID          int       `json:"id"`
	Iid         int       `json:"iid"`
	ProjectID   int       `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DueDate     string    `json:"due_date"`
	StartDate   string    `json:"start_date"`
	WebURL      string    `json:"web_url"`
}
