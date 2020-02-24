package main

import (
	"net/url"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent/integrations/gitlab/api"
	"github.com/pinpt/agent/integrations/pkg/objsender"
	"github.com/pinpt/agent/pkg/commitusers"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

// used for hosted gitlab
func UsersEmails(s *Integration) error {

	userSender, err := objsender.Root(s.agent, sourcecode.UserModelName.String())
	if err != nil {
		return err
	}

	commituserSender, err := objsender.Root(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}

	err = api.PaginateStartAt(s.qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		page, users, err := api.UsersPage(s.qc, paginationParams)
		if err != nil {
			return page, err
		}
		if err = userSender.SetTotal(page.Total); err != nil {
			return page, err
		}
		for _, user := range users {
			cUser := commitusers.CommitUser{
				CustomerID: s.qc.CustomerID,
				Email:      user.Email,
				Name:       user.Name,
				SourceID:   user.Username,
			}
			if err = cUser.Validate(); err != nil {
				return page, err
			}
			if err := commituserSender.Send(cUser); err != nil {
				return page, err
			}

			emails, err := api.UserEmails(s.qc, user.ID)
			if err != nil {
				return page, err
			}
			for _, email := range emails {
				cUser := commitusers.CommitUser{
					CustomerID: s.qc.CustomerID,
					Email:      email,
					Name:       user.Name,
					SourceID:   user.Username,
				}
				if err := cUser.Validate(); err != nil {
					return page, err
				}

				if err := commituserSender.Send(cUser); err != nil {
					return page, err
				}
			}

			sourceUser := sourcecode.User{}
			sourceUser.RefType = s.qc.RefType
			sourceUser.CustomerID = s.qc.CustomerID
			sourceUser.RefID = strconv.FormatInt(user.ID, 10)
			sourceUser.Name = user.Name
			sourceUser.AvatarURL = pstrings.Pointer(user.AvatarURL)
			sourceUser.Username = pstrings.Pointer(user.Username)
			sourceUser.Member = true
			sourceUser.Type = sourcecode.UserTypeHuman
			sourceUser.URL = pstrings.Pointer(user.URL)

			if err := userSender.Send(&sourceUser); err != nil {
				return page, err
			}

		}

		return page, nil
	})

	if err != nil {
		return err
	}

	if err = commituserSender.Done(); err != nil {
		return err
	}

	return userSender.Done()
}
