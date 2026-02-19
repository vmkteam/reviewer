package reviewer

import (
	"testing"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"
)

func TestDBLoad(t *testing.T) {
	t.SkipNow()
	dbc, lg := test.Setup(t)

	lg.Print(t.Context(), "Initialize database")

	p1, _ := test.Prompt(t, dbc, &db.Prompt{
		Title: "Go Review",
		Common: `Дополнительно проверь текст задачи на предмет фикса.
				Если были исправлены ошибки, предположи, что к ним привело и где еще могут быть потенциальные ошибки.`,
		Architecture: "Dave Cheney. Есть ли ошибки в бизнес логике?",
		Code:         "Rob Pike. Обязательно расскажи, что можно улучшить и упросить в данном коде.",
		Security:     "Filippo Valsorda. Проведи ревью безопасности этого MR.",
		Tests:        "Mitchell Hashimoto. Проведи ревью тестов этого MR. Если тестов нет — укажи, какие нужно добавить и почему.",
		StatusID:     db.StatusEnabled,
	})

	lg.Print(t.Context(), "Load prompt", "prompt", p1)

	t1, _ := test.TaskTracker(t, dbc, &db.TaskTracker{
		Title:     "Ютека YouTrack",
		AuthToken: Ptr("xxx"),
		FetchPrompt: `Как получить текст задачи? Номер задачи есть в коммите. Пример номера PLF-731. Как получить по нему информацию.
curl -X GET "https://youtrack/api/issues/PLF-731?fields=\$type,id,summary,description,comments" -H 'Accept: application/json' -H 'Authorization: Bearer {{TOKEN}}' -H 'Cache-Control: no-cache' -H 'Content-Type: application/json'
Ты можешь получить до 10 связанных задач (родительских или в тексте, если нужно больше контекста) по API.`,
		StatusID: db.StatusEnabled,
	})

	lg.Print(t.Context(), "Load taskTracker", "taskTracker", t1)

	pr1, _ := test.Project(t, dbc, &db.Project{
		Title:         "chat/apisrv",
		VcsURL:        "https://gitlab/chat/apisrv",
		Language:      "go",
		ProjectKey:    "93b90214-3b5d-4fa6-b497-f064ff7bf8a9",
		PromptID:      p1.ID,
		TaskTrackerID: Ptr(t1.ID),
		StatusID:      db.StatusEnabled,
	})

	lg.Print(t.Context(), "Load project", "project", pr1)
}
