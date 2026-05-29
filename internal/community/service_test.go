package community

import (
	"context"
	"testing"
)

func TestBoardThreadCommentReactionFlow(t *testing.T) {
	service := NewServiceWithDB(nil)
	ctx := context.Background()

	boards, err := service.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards returned error: %v", err)
	}
	if len(boards) < 3 {
		t.Fatalf("ListBoards returned %d boards, want at least 3", len(boards))
	}
	boardID := boards[0].ID

	thread, err := service.CreateThread(ctx, "user-1", boardID, "A useful reading thread", "This book really rewards slow reading.", "book-1", nil)
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

	threads, err := service.ListThreads(ctx, boardID, 1, 10)
	if err != nil {
		t.Fatalf("ListThreads returned error: %v", err)
	}
	if len(threads) != 1 || threads[0].ID != thread.ID {
		t.Fatalf("ListThreads returned %+v, want thread %q", threads, thread.ID)
	}

	comment, err := service.AddComment(ctx, "user-2", thread.ID, "", "I agree with this read.", nil)
	if err != nil {
		t.Fatalf("AddComment returned error: %v", err)
	}
	reply, err := service.AddComment(ctx, "user-1", thread.ID, comment.ID, "Glad it helped.", nil)
	if err != nil {
		t.Fatalf("AddComment reply returned error: %v", err)
	}
	if reply.ParentCommentID != comment.ID {
		t.Fatalf("reply parent = %q, want %q", reply.ParentCommentID, comment.ID)
	}

	reaction, err := service.React(ctx, "user-2", TargetTypeThread, thread.ID, ReactionLike)
	if err != nil {
		t.Fatalf("React returned error: %v", err)
	}
	if reaction.Status != StatusActive {
		t.Fatalf("reaction status = %q, want %q", reaction.Status, StatusActive)
	}

	got, err := service.GetThread(ctx, thread.ID)
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if got.ReactionCounts[ReactionLike] != 1 {
		t.Fatalf("thread like count = %d, want 1", got.ReactionCounts[ReactionLike])
	}
	if len(got.Comments) != 2 {
		t.Fatalf("thread comments = %d, want 2", len(got.Comments))
	}

	updated, err := service.React(ctx, "user-2", TargetTypeThread, thread.ID, ReactionLove)
	if err != nil {
		t.Fatalf("React update returned error: %v", err)
	}
	if updated.ReactionType != ReactionLove {
		t.Fatalf("updated reaction = %q, want %q", updated.ReactionType, ReactionLove)
	}
	got, err = service.GetThread(ctx, thread.ID)
	if err != nil {
		t.Fatalf("GetThread returned error: %v", err)
	}
	if got.ReactionCounts[ReactionLike] != 0 || got.ReactionCounts[ReactionLove] != 1 {
		t.Fatalf("reaction counts = %+v, want like=0 love=1", got.ReactionCounts)
	}
}

func TestCommunityValidation(t *testing.T) {
	service := NewServiceWithDB(nil)
	ctx := context.Background()

	if _, err := service.CreateThread(ctx, "", "missing", "title", "content", "", nil); err == nil {
		t.Fatal("CreateThread with missing user and board succeeded, want error")
	}
	if _, err := service.ListThreads(ctx, "missing", 1, 10); err == nil {
		t.Fatal("ListThreads for missing board succeeded, want error")
	}
	if _, err := service.AddComment(ctx, "user-1", "missing", "", "content", nil); err == nil {
		t.Fatal("AddComment for missing thread succeeded, want error")
	}
	if _, err := service.React(ctx, "user-1", "unknown", "id", ReactionLike); err == nil {
		t.Fatal("React with unknown target type succeeded, want error")
	}
}

func TestAttachmentFlow(t *testing.T) {
	service := NewServiceWithDB(nil)
	ctx := context.Background()

	boards, err := service.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}
	boardID := boards[0].ID

	// Create a pending attachment
	att, err := service.CreateAttachment(ctx, CreateAttachmentInput{
		OwnerUserID:     "user-1",
		FileType:        FileTypeImage,
		StorageProvider: "local",
		StorageKey:      "test/image.jpg",
		PublicURL:       "https://example.com/test/image.jpg",
		MIMEType:        "image/jpeg",
		FileSize:        12345,
		Width:           800,
		Height:          600,
		ChecksumSHA256:  "abc123",
	})
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}
	if att.Status != StatusPending {
		t.Fatalf("attachment status = %q, want %q", att.Status, StatusPending)
	}

	// Create thread with attachment
	thread, err := service.CreateThread(ctx, "user-1", boardID, "Thread with image", "Content", "", []string{att.ID})
	if err != nil {
		t.Fatalf("CreateThread with attachment: %v", err)
	}
	if len(thread.Attachments) != 1 {
		t.Fatalf("thread attachments = %d, want 1", len(thread.Attachments))
	}
	if thread.Attachments[0].ID != att.ID {
		t.Fatalf("attachment ID = %q, want %q", thread.Attachments[0].ID, att.ID)
	}

	// Create comment with attachment
	att2, err := service.CreateAttachment(ctx, CreateAttachmentInput{
		OwnerUserID:     "user-2",
		FileType:        FileTypeImage,
		StorageProvider: "local",
		StorageKey:      "test/image2.png",
		PublicURL:       "https://example.com/test/image2.png",
		MIMEType:        "image/png",
		FileSize:        54321,
		Width:           1024,
		Height:          768,
		ChecksumSHA256:  "def456",
	})
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	comment, err := service.AddComment(ctx, "user-2", thread.ID, "", "Nice image!", []string{att2.ID})
	if err != nil {
		t.Fatalf("AddComment with attachment: %v", err)
	}
	if len(comment.Attachments) != 1 {
		t.Fatalf("comment attachments = %d, want 1", len(comment.Attachments))
	}

	// Verify thread detail includes attachments on comments
	got, err := service.GetThread(ctx, thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("thread detail attachments = %d, want 1", len(got.Attachments))
	}
	if len(got.Comments) != 1 || len(got.Comments[0].Attachments) != 1 {
		t.Fatalf("comment attachments not hydrated")
	}
}

func TestAttachmentLimitExceeded(t *testing.T) {
	service := NewServiceWithDB(nil)
	ctx := context.Background()

	boards, err := service.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards: %v", err)
	}

	// Try creating thread with too many attachment IDs
	ids := make([]string, MaxThreadAttachments+1)
	for i := range ids {
		ids[i] = "fake"
	}
	_, err = service.CreateThread(ctx, "user-1", boards[0].ID, "Title", "Content", "", ids)
	if err == nil {
		t.Fatal("CreateThread with too many attachments succeeded, want error")
	}
}
