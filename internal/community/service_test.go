package community

import "testing"

func TestBoardThreadCommentReactionFlow(t *testing.T) {
	service := NewService()

	boards := service.ListBoards()
	if len(boards) < 3 {
		t.Fatalf("ListBoards returned %d boards, want at least 3", len(boards))
	}
	boardID := boards[0].ID

	thread, err := service.CreateThread("user-1", boardID, "A useful reading thread", "This book really rewards slow reading.", "book-1")
	if err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("CreateThread returned empty ID")
	}
	if thread.Status != StatusActive {
		t.Fatalf("thread status = %q, want %q", thread.Status, StatusActive)
	}
	if thread.OptionalBookID != "book-1" {
		t.Fatalf("OptionalBookID = %q, want book-1", thread.OptionalBookID)
	}

	threads, err := service.ListThreads(boardID, 1, 10)
	if err != nil {
		t.Fatalf("ListThreads returned error: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != thread.ID {
		t.Fatalf("ListThreads returned %+v, want thread %q", threads, thread.ID)
	}

	comment, err := service.AddComment("user-2", thread.ID, "", "I agree with this read.")
	if err != nil {
		t.Fatalf("AddComment returned error: %v", err)
	}
	reply, err := service.AddComment("user-1", thread.ID, comment.ID, "Glad it helped.")
	if err != nil {
		t.Fatalf("AddComment reply returned error: %v", err)
	}
	if reply.ParentCommentID != comment.ID {
		t.Fatalf("reply parent = %q, want %q", reply.ParentCommentID, comment.ID)
	}

	reaction, err := service.React("user-2", TargetTypeThread, thread.ID, ReactionLike)
	if err != nil {
		t.Fatalf("React returned error: %v", err)
	}
	if reaction.Status != StatusActive {
		t.Fatalf("reaction status = %q, want %q", reaction.Status, StatusActive)
	}

	got, err := service.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if got.ReactionCounts[ReactionLike] != 1 {
		t.Fatalf("thread like count = %d, want 1", got.ReactionCounts[ReactionLike])
	}
	if len(got.Comments) != 2 {
		t.Fatalf("thread comments = %d, want 2", len(got.Comments))
	}

	updated, err := service.React("user-2", TargetTypeThread, thread.ID, ReactionLove)
	if err != nil {
		t.Fatalf("React update returned error: %v", err)
	}
	if updated.ReactionType != ReactionLove {
		t.Fatalf("updated reaction = %q, want %q", updated.ReactionType, ReactionLove)
	}
	got, err = service.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if got.ReactionCounts[ReactionLike] != 0 || got.ReactionCounts[ReactionLove] != 1 {
		t.Fatalf("reaction counts = %+v, want like=0 love=1", got.ReactionCounts)
	}
}

func TestCommunityValidation(t *testing.T) {
	service := NewService()

	if _, err := service.CreateThread("", "missing", "title", "content", ""); err == nil {
		t.Fatal("CreateThread with missing user and board succeeded, want error")
	}
	if _, err := service.ListThreads("missing", 1, 10); err == nil {
		t.Fatal("ListThreads for missing board succeeded, want error")
	}
	if _, err := service.AddComment("user-1", "missing", "", "content"); err == nil {
		t.Fatal("AddComment for missing thread succeeded, want error")
	}
	if _, err := service.React("user-1", "unknown", "id", ReactionLike); err == nil {
		t.Fatal("React with unknown target type succeeded, want error")
	}
}

func TestListUserThreadsReturnsOnlyThatUsersActiveThreads(t *testing.T) {
	service := NewService()
	boardID := service.ListBoards()[0].ID

	first, err := service.CreateThread("user-1", boardID, "First", "First post content", "")
	if err != nil {
		t.Fatalf("CreateThread first returned error: %v", err)
	}
	if _, err := service.CreateThread("user-2", boardID, "Other", "Other user content", ""); err != nil {
		t.Fatalf("CreateThread other returned error: %v", err)
	}
	second, err := service.CreateThread("user-1", boardID, "Second", "Second post content", "")
	if err != nil {
		t.Fatalf("CreateThread second returned error: %v", err)
	}

	threads, err := service.ListUserThreads("user-1", 1, 10)
	if err != nil {
		t.Fatalf("ListUserThreads returned error: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("ListUserThreads returned %d threads, want 2", len(threads))
	}
	if threads[0].ID != second.ID || threads[1].ID != first.ID {
		t.Fatalf("ListUserThreads order = [%s, %s], want newest first [%s, %s]", threads[0].ID, threads[1].ID, second.ID, first.ID)
	}
}

func TestAddCommentRejectsSecondLevelReply(t *testing.T) {
	service := NewService()
	boardID := service.ListBoards()[0].ID
	thread, err := service.CreateThread("user-1", boardID, "Thread", "Thread content", "")
	if err != nil {
		t.Fatalf("CreateThread returned error: %v", err)
	}
	comment, err := service.AddComment("user-2", thread.ID, "", "Top-level comment")
	if err != nil {
		t.Fatalf("AddComment returned error: %v", err)
	}
	reply, err := service.AddComment("user-1", thread.ID, comment.ID, "First-level reply")
	if err != nil {
		t.Fatalf("AddComment reply returned error: %v", err)
	}

	if _, err := service.AddComment("user-2", thread.ID, reply.ID, "Second-level reply"); err == nil {
		t.Fatal("AddComment accepted a second-level reply, want error")
	}
}
