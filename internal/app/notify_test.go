package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// The outcome notice names the way out for each failure, so the row's
// reader can act on it (or trust the delivery).
func TestNotificationResultNotice(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "测试通知已发送"},
		{mygo.ErrNotificationsDenied, "系统设置"},
		{errors.ErrUnsupported, "mygo"},
		{errors.New("boom"), "通知发送失败"},
	}
	for _, tc := range cases {
		got := notificationResultNotice(tc.err)
		if !strings.Contains(got, tc.want) {
			t.Errorf("notice(%v) = %q, want it to mention %q", tc.err, got, tc.want)
		}
	}
}

// A test notification on a run that cannot show any still reports an
// outcome, instead of the button doing nothing.
func TestSendTestNotificationUnsupported(t *testing.T) {
	restore := notificationsSupported
	notificationsSupported = func() bool { return false }
	defer func() { notificationsSupported = restore }()

	a, err := NewForTest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if text, at := a.NotificationNotice(); text != "" || !at.IsZero() {
		t.Fatalf("notice before any test = %q at %v", text, at)
	}

	a.SendTestNotification()

	text, at := a.NotificationNotice()
	if text != notificationResultNotice(errors.ErrUnsupported) {
		t.Errorf("notice = %q", text)
	}
	if at.IsZero() {
		t.Error("notice has no time")
	}
}
