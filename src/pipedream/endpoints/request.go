package endpoints

import (
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pipedream/apps"
	"pipedream/providers"
)

type LastRequest struct {
	repos    map[string]time.Time
	idle     time.Duration
	reserved *Reserved
}

func (r *LastRequest) Setup(provider providers.Provider) error {
	// populate last request
	applications, err := provider.ListApps()
	if err != nil {
		return err
	}
	for _, app := range applications {
		r.AddRequest(app)
	}

	r.StartTicker(provider)

	return nil
}

func (r *LastRequest) AddRequest(app apps.App) {
	key := app.String()
	r.repos[key] = time.Now()
}

func (r *LastRequest) Remove(app apps.App) {
	delete(r.repos, app.String())
}

func (r *LastRequest) Get(app apps.App) time.Time {
	key := app.String()
	return r.repos[key]
}

func (r *LastRequest) StartTicker(provider providers.Provider) {
	ticker := time.NewTicker(time.Second * 60)
	go func() {
		for _ = range ticker.C {
			stale := r.GetStaleApps()
			for _, app := range stale {
				org, repo, branch := app[0], app[1], app[2]
				app := apps.NewApp(org, repo, branch)
				err := provider.Stop(app)
				if err != nil {
					log.Printf("Error stopping app: %+v", err)
				} else {
					r.Remove(app)
				}
			}
		}
	}()
}

func (r *LastRequest) GetStaleApps() [][]string {
	stale := make([][]string, 0)
	for repo, lastRequest := range r.repos {
		// skip apps that are "alwaysOn"
		parts := strings.Split(repo, ".")
		org, repo, branch := parts[0], parts[1], parts[2]
		app := apps.NewApp(org, repo, branch)

		if ok, _ := r.reserved.IsReserved(app); ok {
			continue
		}

		duration := time.Since(lastRequest)
		if duration > r.idle {
			stale = append(stale, parts)
		}
	}
	return stale
}

func (r LastRequest) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		org := c.Param("org")
		repo := c.Param("repo")
		branch := c.Param("branch")
		if org != "" && repo != "" && branch != "" {
			app := apps.NewApp(org, repo, branch)
			r.AddRequest(app)
		}
	}
}
