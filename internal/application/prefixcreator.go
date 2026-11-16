package application

import (
	"context"

	"github.com/pan-daniel/line-proton/internal/domain/prefix"
)

type PrefixCreator struct {
	wine     Wine
	contents prefix.Contents
}

func NewPrefixCreator(wine Wine, contents prefix.Contents) PrefixCreator {
	return PrefixCreator{wine: wine, contents: contents}
}

func (creator PrefixCreator) Create(ctx context.Context) error {
	if creator.contents.Exists("system.reg") {
		return nil
	}
	return creator.wine.Run(ctx, "wineboot", "-u")
}
