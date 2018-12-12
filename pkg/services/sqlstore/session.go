package sqlstore

import (
	"context"
	"reflect"

	"github.com/go-xorm/xorm"
)

type DBSession struct {
	*xorm.Session
	events []interface{}
}

type dbTransactionFunc func(sess *DBSession) error

func (sess *DBSession) publishAfterCommit(msg interface{}) {
	sess.events = append(sess.events, msg)
}

func newSession() *DBSession {
	return &DBSession{Session: x.NewSession()}
}

func startSession(ctx context.Context, engine *xorm.Engine, beginTran bool) (*DBSession, error) {
	value := ctx.Value(ContextSessionName)
	var sess *DBSession
	sess, ok := value.(*DBSession)

	if ok {
		return sess, nil
	}

	newSess := &DBSession{Session: engine.NewSession()}
	if beginTran {
		err := newSess.Begin()
		if err != nil {
			return nil, err
		}
	}
	return newSess, nil
}

func withDbSession(ctx context.Context, callback dbTransactionFunc) error {
	sess, err := startSession(ctx, x, false)
	if err != nil {
		return err
	}

	return callback(sess)
}

func (sess *DBSession) InsertId(bean interface{}) (int64, error) {
	table := sess.DB().Mapper.Obj2Table(getTypeName(bean))

	dialect.PreInsertId(table, sess.Session)

	id, err := sess.Session.InsertOne(bean)

	dialect.PostInsertId(table, sess.Session)

	return id, err
}

func getTypeName(bean interface{}) (res string) {
	t := reflect.TypeOf(bean)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
