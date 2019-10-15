package main

import (
	"net/url"
	"strconv"

	"github.com/hashicorp/go-hclog"
	"github.com/pinpt/agent.next/integrations/gitlab/api"
	"github.com/pinpt/agent.next/integrations/pkg/objsender2"
	"github.com/pinpt/agent.next/pkg/commitusers"
	pstrings "github.com/pinpt/go-common/strings"
	"github.com/pinpt/integration-sdk/sourcecode"
)

func UsersEmails(s *Integration) error {

	userSender, err := objsender2.Root(s.agent, sourcecode.UserModelName.String())
	if err != nil {
		return err
	}

	commituserSender, err := objsender2.Root(s.agent, commitusers.TableName)
	if err != nil {
		return err
	}

	err = api.PaginateStartAt(s.qc.Logger, func(log hclog.Logger, paginationParams url.Values) (page api.PageInfo, _ error) {
		page, users, err := api.UsersPage(s.qc, paginationParams)
		if err != nil {
			return page, err
		}
		for _, user := range users {
			cUser := commitusers.CommitUser{
				CustomerID: s.qc.CustomerID,
				Email:      user.Email,
				Name:       user.Name,
				SourceID:   user.Username,
			}
			err = cUser.Validate()
			if err != nil {
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
				err := cUser.Validate()
				if err != nil {
					return page, err
				}

				if err := commituserSender.Send(cUser); err != nil {
					return page, err
				}
			}

			sourceUser := sourcecode.User{}
			sourceUser.RefType = s.qc.RefType
			sourceUser.Email = pstrings.Pointer(user.Email)
			sourceUser.CustomerID = s.qc.CustomerID
			sourceUser.RefID = strconv.FormatInt(user.ID, 10)
			sourceUser.Name = user.Name
			sourceUser.AvatarURL = pstrings.Pointer(user.AvatarURL)
			sourceUser.Username = pstrings.Pointer(user.Username)
			sourceUser.Member = true
			sourceUser.Type = sourcecode.UserTypeHuman
			sourceUser.AssociatedRefID = pstrings.Pointer(user.Username)

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
