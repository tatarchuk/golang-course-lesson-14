package chat_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"homework/internal/chat"
	"homework/internal/model"
	"homework/internal/repository"
)

// recorder is a Publisher that remembers every notice.
type recorder struct {
	mu    sync.Mutex
	rooms []string
	msgs  []chat.Message
}

func (r *recorder) Publish(room string, m chat.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rooms = append(r.rooms, room)
	r.msgs = append(r.msgs, m)
}

// fakeAlbums serves the album calls the decorator makes; any other call fails the test.
type fakeAlbums struct {
	t      *testing.T
	create func(model.AlbumInput) (model.Album, error)
	get    func(int64) (model.Album, error)
	del    func(int64) error
}

var _ repository.AlbumRepository = (*fakeAlbums)(nil)

func (f *fakeAlbums) Create(_ context.Context, in model.AlbumInput) (model.Album, error) {
	return f.create(in)
}

func (f *fakeAlbums) Get(_ context.Context, id int64) (model.Album, error) { return f.get(id) }

func (f *fakeAlbums) List(context.Context, repository.ListOptions) ([]model.Album, error) {
	f.t.Fatal("unexpected List call")
	return nil, nil
}

func (f *fakeAlbums) Update(context.Context, int64, model.AlbumInput) (model.Album, error) {
	f.t.Fatal("unexpected Update call")
	return model.Album{}, nil
}

func (f *fakeAlbums) Delete(_ context.Context, id int64) error { return f.del(id) }

var blueTrain = model.Album{ID: 11, Title: "Blue Train", Label: "Blue Note", Genre: model.Genre{ID: 1, Name: "jazz"}}

// TestNotifyOnCreate is the REST side of the chat (slide 41's one new line, as a
// repository decorator): a created album is announced in general.
func TestNotifyOnCreate(t *testing.T) {
	rec := &recorder{}
	albums := chat.NotifyAlbums(&fakeAlbums{t: t, create: func(model.AlbumInput) (model.Album, error) { return blueTrain, nil }}, rec)
	a, err := albums.Create(context.Background(), model.AlbumInput{})
	if err != nil || a.ID != 11 {
		t.Fatalf("Create = %+v, %v", a, err)
	}
	want := `Album "Blue Train" (Blue Note, jazz) was added to the catalogue`
	if len(rec.msgs) != 1 || rec.rooms[0] != chat.RoomGeneral || rec.msgs[0].Type != chat.TypeNotice || rec.msgs[0].Text != want {
		t.Fatalf("published %v %+v, want one notice %q in general", rec.rooms, rec.msgs, want)
	}
}

func TestNotifyOnDelete(t *testing.T) {
	rec := &recorder{}
	albums := chat.NotifyAlbums(&fakeAlbums{t: t,
		get: func(int64) (model.Album, error) { return blueTrain, nil },
		del: func(int64) error { return nil },
	}, rec)
	if err := albums.Delete(context.Background(), 11); err != nil {
		t.Fatal(err)
	}
	want := `Album "Blue Train" was removed from the catalogue`
	if len(rec.msgs) != 1 || rec.rooms[0] != chat.RoomGeneral || rec.msgs[0].Text != want {
		t.Fatalf("published %v %+v, want one notice %q in general", rec.rooms, rec.msgs, want)
	}
}

// TestNoNoticeOnError: a write that fails announces nothing and returns its error unchanged.
func TestNoNoticeOnError(t *testing.T) {
	rec := &recorder{}
	albums := chat.NotifyAlbums(&fakeAlbums{t: t,
		create: func(model.AlbumInput) (model.Album, error) { return model.Album{}, repository.ErrConflict },
		get:    func(int64) (model.Album, error) { return model.Album{}, repository.ErrNotFound },
		del:    func(int64) error { return repository.ErrNotFound },
	}, rec)
	if _, err := albums.Create(context.Background(), model.AlbumInput{}); !errors.Is(err, repository.ErrConflict) {
		t.Errorf("Create error %v, want ErrConflict", err)
	}
	if err := albums.Delete(context.Background(), 404); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete error %v, want ErrNotFound", err)
	}
	if len(rec.msgs) != 0 {
		t.Fatalf("published %+v, want nothing", rec.msgs)
	}
}
