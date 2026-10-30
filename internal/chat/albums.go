package chat

import (
	"context"
	"fmt"
	"strconv"

	"homework/internal/model"
	"homework/internal/repository"
)

// Publisher is what the album notices need from the hub.
type Publisher interface {
	Publish(room string, m Message)
}

// NotifyAlbums wraps the album repository so that every album created or deleted is
// announced in the general room. It is slide 41's "one new line in the service" for this
// app, which has no service layer: as a decorator of the repository the API and the web
// handlers stay unchanged, and a failed write announces nothing.
func NotifyAlbums(next repository.AlbumRepository, p Publisher) repository.AlbumRepository {
	return &notifyingAlbums{AlbumRepository: next, p: p}
}

type notifyingAlbums struct {
	repository.AlbumRepository
	p Publisher
}

func (n *notifyingAlbums) Create(ctx context.Context, in model.AlbumInput) (model.Album, error) {
	a, err := n.AlbumRepository.Create(ctx, in)
	if err != nil {
		return a, err
	}
	n.p.Publish(RoomGeneral, Message{Type: TypeNotice,
		Text: fmt.Sprintf("Album %q (%s, %s) was added to the catalogue", a.Title, a.Label, a.Genre.Name)})
	return a, nil
}

// Delete reads the album first, so the notice can name it.
func (n *notifyingAlbums) Delete(ctx context.Context, id int64) error {
	a, getErr := n.AlbumRepository.Get(ctx, id)
	if err := n.AlbumRepository.Delete(ctx, id); err != nil {
		return err
	}
	name := "#" + strconv.FormatInt(id, 10)
	if getErr == nil {
		name = strconv.Quote(a.Title)
	}
	n.p.Publish(RoomGeneral, Message{Type: TypeNotice, Text: "Album " + name + " was removed from the catalogue"})
	return nil
}
