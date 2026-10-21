package auth

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
)

// UpdateAccount changes the e-mail or the password of the session's account after checking the
// current password (M8, A-2). The base of the milestone answers model.ErrNotImplemented (501).
// M8 lane S1 implements this.
func (s *Service) UpdateAccount(context.Context, model.Session, model.AccountUpdate) (model.Session, error) {
	return model.Session{}, model.ErrNotImplemented
}
