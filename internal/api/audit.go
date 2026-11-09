package api

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nermline/Melotrack-Creator/internal/audit"
	"github.com/nermline/Melotrack-Creator/internal/auth"
	"github.com/nermline/Melotrack-Creator/internal/store"
)

// record writes an action-log entry on behalf of whoever made the request.
func (s *Server) record(c *gin.Context, action string, attrs ...any) {
	audit.Record(auth.UserOf(c).ID(), action, append([]any{"ip", c.ClientIP()}, attrs...)...)
}

// changeList describes an edit as "field: old → new" pairs.
type changeList []string

func (l *changeList) add(field string, old, new any) {
	if o, n := logValue(old), logValue(new); o != n {
		*l = append(*l, field+": "+o+" → "+n)
	}
}

func (l changeList) String() string { return strings.Join(l, "; ") }

func logValue(v any) string {
	switch x := v.(type) {
	case string:
		// «» rather than quotes: the log line quotes the whole value again.
		return "«" + strings.Join(strings.Fields(truncateRunes(x, 80)), " ") + "»"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case *float64:
		if x == nil {
			return "—"
		}
		return strconv.FormatFloat(*x, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// itemLabel names a song in the log: "category #position «answer»".
func (s *Server) itemLabel(it *store.Item) string {
	var cat store.Category
	s.db.Select("title").First(&cat, it.CategoryID)
	return fmt.Sprintf("%s #%d «%s»", cat.Title, it.Position+1, truncateRunes(it.Answer, 80))
}
