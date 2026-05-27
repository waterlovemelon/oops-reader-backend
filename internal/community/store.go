package community

type Store interface {
	ListBoards() ([]Board, error)
	ListThreads(boardID string, page, pageSize int) ([]Thread, error)
	ListUserThreads(userID string, page, pageSize int) ([]Thread, error)
	CreateThread(userID, boardID, title, content, optionalBookID string) (Thread, error)
	GetThread(id string) (Thread, error)
	AddComment(userID, threadID, parentCommentID, content string) (Comment, error)
	React(userID, targetType, targetID, reactionType string) (Reaction, error)
}
