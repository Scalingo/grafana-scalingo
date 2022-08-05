package comments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/comments/commentmodel"
)

func commentsToDto(items []*commentmodel.Comment, userMap map[int64]*commentmodel.CommentUser) []*commentmodel.CommentDto {
	result := make([]*commentmodel.CommentDto, 0, len(items))
	for _, m := range items {
		result = append(result, commentToDto(m, userMap))
	}
	return result
}

func commentToDto(comment *commentmodel.Comment, userMap map[int64]*commentmodel.CommentUser) *commentmodel.CommentDto {
	var u *commentmodel.CommentUser
	if comment.UserId > 0 {
		var ok bool
		u, ok = userMap[comment.UserId]
		if !ok {
			// TODO: handle this gracefully?
			u = &commentmodel.CommentUser{
				Id: comment.UserId,
			}
		}
	}
	return comment.ToDTO(u)
}

func searchUserToCommentUser(searchUser *models.UserSearchHitDTO) *commentmodel.CommentUser {
	if searchUser == nil {
		return nil
	}
	return &commentmodel.CommentUser{
		Id:        searchUser.Id,
		Name:      searchUser.Name,
		Login:     searchUser.Login,
		Email:     searchUser.Email,
		AvatarUrl: dtos.GetGravatarUrl(searchUser.Email),
	}
}

type UserIDFilter struct {
	userIDs []int64
}

func NewIDFilter(userIDs []int64) models.Filter {
	return &UserIDFilter{userIDs: userIDs}
}

func (a *UserIDFilter) WhereCondition() *models.WhereCondition {
	return nil
}

func (a *UserIDFilter) JoinCondition() *models.JoinCondition {
	return nil
}

func (a *UserIDFilter) InCondition() *models.InCondition {
	return &models.InCondition{
		Condition: "u.id",
		Params:    a.userIDs,
	}
}

type GetCmd struct {
	ObjectType string `json:"objectType"`
	ObjectID   string `json:"objectId"`
	Limit      uint   `json:"limit"`
	BeforeId   int64  `json:"beforeId"`
}

type CreateCmd struct {
	ObjectType string `json:"objectType"`
	ObjectID   string `json:"objectId"`
	Content    string `json:"content"`
}

var ErrPermissionDenied = errors.New("permission denied")

func (s *Service) Create(ctx context.Context, orgID int64, signedInUser *models.SignedInUser, cmd CreateCmd) (*commentmodel.CommentDto, error) {
	ok, err := s.permissions.CheckWritePermissions(ctx, orgID, signedInUser, cmd.ObjectType, cmd.ObjectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrPermissionDenied
	}

	userMap := make(map[int64]*commentmodel.CommentUser, 1)
	if signedInUser.UserId > 0 {
		userMap[signedInUser.UserId] = &commentmodel.CommentUser{
			Id:        signedInUser.UserId,
			Name:      signedInUser.Name,
			Login:     signedInUser.Login,
			Email:     signedInUser.Email,
			AvatarUrl: dtos.GetGravatarUrl(signedInUser.Email),
		}
	}

	m, err := s.storage.Create(ctx, orgID, cmd.ObjectType, cmd.ObjectID, signedInUser.UserId, cmd.Content)
	if err != nil {
		return nil, err
	}
	mDto := commentToDto(m, userMap)
	e := commentmodel.Event{
		Event:          commentmodel.EventCommentCreated,
		CommentCreated: mDto,
	}
	eventJSON, _ := json.Marshal(e)
	_ = s.live.Publish(orgID, fmt.Sprintf("grafana/comment/%s/%s", cmd.ObjectType, cmd.ObjectID), eventJSON)
	return mDto, nil
}

func (s *Service) Get(ctx context.Context, orgID int64, signedInUser *models.SignedInUser, cmd GetCmd) ([]*commentmodel.CommentDto, error) {
	ok, err := s.permissions.CheckReadPermissions(ctx, orgID, signedInUser, cmd.ObjectType, cmd.ObjectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrPermissionDenied
	}

	messages, err := s.storage.Get(ctx, orgID, cmd.ObjectType, cmd.ObjectID, GetFilter{
		Limit:    cmd.Limit,
		BeforeID: cmd.BeforeId,
	})
	if err != nil {
		return nil, err
	}

	userIds := make([]int64, 0, len(messages))
	for _, m := range messages {
		if m.UserId <= 0 {
			continue
		}
		userIds = append(userIds, m.UserId)
	}

	// NOTE: probably replace with comment and user table join.
	query := &models.SearchUsersQuery{
		Query:        "",
		Page:         0,
		Limit:        len(userIds),
		SignedInUser: signedInUser,
		Filters:      []models.Filter{NewIDFilter(userIds)},
	}
	if err := s.sqlStore.SearchUsers(ctx, query); err != nil {
		return nil, err
	}

	userMap := make(map[int64]*commentmodel.CommentUser, len(query.Result.Users))
	for _, v := range query.Result.Users {
		userMap[v.Id] = searchUserToCommentUser(v)
	}

	result := commentsToDto(messages, userMap)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Id < result[j].Id
	})
	return result, nil
}
