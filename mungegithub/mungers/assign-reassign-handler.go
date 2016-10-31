/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mungers

import (
	"strings"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/mungers/mungerutil"
	"k8s.io/kubernetes/pkg/util/sets"

	"fmt"
	"github.com/golang/glog"
	goGithub "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

const (
	OWNER            = "kubernetes"
	assign_command   = "/assign"
	reassign_command = "/reassign"

	notReviewerInTree = "%v commented /assign on a PR but it looks you are not list in the OWNERs file as a reviewer for the files in this PR"
)

// AssignReassignHandler will
// - will assign a github user to a PR if they comment "/assign"
// - will unassign a github user to a PR if they comment "/reassign"
type AssignReassignHandler struct {
	features   *features.Features
	checkValid bool
}

func init() {
	dh := AssignReassignHandler{}
	RegisterMungerOrDie(dh)
}

// Name is the name usable in --pr-mungers
func (AssignReassignHandler) Name() string { return "assign-reassign-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (AssignReassignHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (AssignReassignHandler) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (AssignReassignHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (h AssignReassignHandler) AddFlags(cmd *cobra.Command, config *github.Config) {
	cmd.Flags().BoolVar(&h.checkValid, "check-valid-reviewer", true, "Flag indicating whether to allow any one to be assigned or just OWNERs files reviewers")
}

// Munge is the workhorse the will actually make updates to the PR
func (h AssignReassignHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	comments, err := obj.ListComments()
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
		return
	}

	toAssign, toUnassign, err := h.assignOrRemove(obj, comments, true)
	if err != nil {
		return
	}
	//assign and unassign reviewers as necessary
	for _, username := range toAssign.List() {
		obj.AssignPR(username)
	}
	if len(toUnassign) > 0 {
		is := goGithub.IssuesService{}
		is.RemoveAssignees(OWNER, *obj.Issue.Repository.Name, *obj.Issue.Number, toUnassign.List())
	}
}

//assignOrRemove checks to see when someone comments "/assign" or "/reassign"
// "/assign" self assigns the PR
// "/reassign" unassignes the commenter and reassigns to someone else
// [TODO] "/reassign <github handle>" reassign to this person
func (h *AssignReassignHandler) assignOrRemove(obj *github.MungeObject, comments []*goGithub.IssueComment, checkValid bool) (toAssign, toUnassign sets.String, _ error) {

	toAssign = sets.String{}
	toUnassign = sets.String{}
	potential_owners := weightMap{}
	if checkValid {
		fileList, err := obj.ListFiles()
		if err != nil {
			glog.Error("Could not list the files for PR %v", obj.Issue.Number)
			return toAssign, toUnassign, err
		}
		//get all the people that could potentially own the file based on the blunderbuss.go implementation
		potential_owners, _ = getPotentialOwners(obj, h.features, fileList)
	}
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if !mungerutil.IsValidUser(comment.User) {
			continue
		}

		fields := getFields(*comment.Body)
		if isDibsComment(fields) {
			//check if they are a valid reviewer if so, assign the user. if not, explain why
			if !checkValid || isValidReviewer(potential_owners, comment.User) {
				glog.Infof("Assigning %v to review PR#%v", *comment.User.Login, obj.Issue.Number)
				toAssign.Insert(*comment.User.Login)
			} else {
				//inform user that they are not a valid reviewer
				obj.WriteComment(fmt.Sprintf(notReviewerInTree, comment.User.String()))
			}
		}
		if isReassignComment(fields) && isAssignee(obj.Issue.Assignees, comment.User) {
			//check if they are already an assigned reviewer. if so, remove them.  if not, do nothing.
			glog.Infof("Removing %v as an reviewer for PR#%v", *comment.User.Login, obj.Issue.Number)
			toUnassign.Insert(*comment.User.Login)
		}

	}
	return toAssign, toUnassign, nil
}

func isValidReviewer(potential_owners weightMap, commenter *goGithub.User) bool {
	if _, ok := potential_owners[commenter.String()]; ok {
		return true
	}
	return false
}

func isAssignee(assignees []*goGithub.User, someUser *goGithub.User) bool {
	for _, assignee := range assignees {
		//remove the assignee
		if assignee.Login == nil || someUser.Login == nil {
			continue
		}
		if *assignee.Login == *someUser.Login && someUser.ID == assignee.ID {
			return true
		}
	}
	return false
}

func isDibsComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == assign_command
}

func isReassignComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == reassign_command
}
