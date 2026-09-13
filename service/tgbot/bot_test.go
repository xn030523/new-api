package tgbot

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/tgclient"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// telegramStub 是一个最小的 Telegram API 替身，记录调用了哪些方法。
type telegramStub struct {
	mu      sync.Mutex
	methods []string
}

func (t *telegramStub) handler(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	t.methods = append(t.methods, parts[len(parts)-1])
	t.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
}

func (t *telegramStub) called(method string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, m := range t.methods {
		if m == method {
			return true
		}
	}
	return false
}

// setupTestBot 起一个指向 stub 的 BotService 和内存 users 表。
func setupTestBot(t *testing.T) (*BotService, *telegramStub) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })

	stub := &telegramStub{}
	server := httptest.NewServer(http.HandlerFunc(stub.handler))
	t.Cleanup(server.Close)

	s := NewBotService()
	s.client = tgclient.NewClientWithBaseURL("test-token", server.URL)
	s.features = map[string]*BotFeature{
		FeatureAdduser: {
			Name:    FeatureAdduser,
			Enabled: true,
			Targets: `[{"chat_id":42,"enabled":true}]`,
			Settings: `{"allowed_groups":["g1"],"default_group":"g1",` +
				`"remarks":["vip"],"initial_quota":1000}`,
		},
	}
	return s, stub
}

// 生产实测回归：adduser 最后一步是点备注按钮（回调），而完成创建只挂在
// 文本消息路径上，点完按钮会话停在 step 4 无人处理，按钮一直转圈。
func TestHandleCallbackCompletesAdduserOnRemarkSelection(t *testing.T) {
	s, stub := setupTestBot(t)
	const userID int64 = 7

	s.conversations.Store(userID, &ConversationState{
		Command: "adduser",
		Step:    3,
		Data: map[string]any{
			"adduser_group": "g1",
			"username":      "newbie",
			"password":      "password123",
		},
	})

	s.handleCallback(&tgclient.CallbackQuery{
		ID:      "cb1",
		From:    &tgclient.User{ID: userID},
		Message: &tgclient.Message{Chat: tgclient.Chat{ID: 42}},
		Data:    "adduser_remark:vip",
	})

	var user model.User
	require.NoError(t, model.DB.Where("username = ?", "newbie").First(&user).Error)
	assert.Equal(t, "g1", user.Group)
	assert.Equal(t, "vip", user.Remark)
	assert.NotEmpty(t, user.Password)
	assert.NotEqual(t, "password123", user.Password, "密码必须哈希存储")

	_, alive := s.conversations.Load(userID)
	assert.False(t, alive, "会话必须在完成后删除")
	assert.True(t, stub.called("answerCallbackQuery"), "必须应答回调，否则按钮一直转圈")
}

// 点分组按钮（中间步骤）不能触发创建，只应推进到下一步提问。
func TestHandleCallbackAdduserGroupOnlyAdvances(t *testing.T) {
	s, stub := setupTestBot(t)
	const userID int64 = 7

	s.conversations.Store(userID, &ConversationState{
		Command: "adduser",
		Step:    0,
		Data:    map[string]any{},
	})

	s.handleCallback(&tgclient.CallbackQuery{
		ID:      "cb1",
		From:    &tgclient.User{ID: userID},
		Message: &tgclient.Message{Chat: tgclient.Chat{ID: 42}},
		Data:    "adduser_group:g1",
	})

	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
	assert.Zero(t, count, "只选了分组就创建用户是不对的")

	stateValue, alive := s.conversations.Load(userID)
	require.True(t, alive, "中间步骤会话必须保留")
	state := stateValue.(*ConversationState)
	assert.Equal(t, 1, state.Step)
	assert.Equal(t, "g1", state.Data["adduser_group"])
	assert.True(t, stub.called("answerCallbackQuery"))
	assert.True(t, stub.called("sendMessage"), "下一步是让用户输入用户名")
}

// 会话过期（比如重启后内存会话丢了）要点名拒绝，而不是悄悄无事发生。
func TestHandleCallbackRejectsExpiredConversation(t *testing.T) {
	s, stub := setupTestBot(t)

	s.handleCallback(&tgclient.CallbackQuery{
		ID:      "cb1",
		From:    &tgclient.User{ID: 7},
		Message: &tgclient.Message{Chat: tgclient.Chat{ID: 42}},
		Data:    "adduser_remark:vip",
	})

	assert.True(t, stub.called("answerCallbackQuery"))
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Count(&count).Error)
	assert.Zero(t, count)
}
