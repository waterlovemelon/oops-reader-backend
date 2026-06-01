package catalog

import "context"

// CommentsStore defines persistence operations for catalog book comments.
type CommentsStore interface {
	ListComments(ctx context.Context, bookKey string, limit, offset int) ([]BookComment, int, error)
	CreateComment(ctx context.Context, comment BookComment) (BookComment, error)
	CountPublished(ctx context.Context, bookKey string) (int, error)
}
