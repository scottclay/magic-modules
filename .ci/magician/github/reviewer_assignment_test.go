/*
* Copyright 2023 Google LLC. All Rights Reserved.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */
package github

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golang.org/x/exp/slices"
)

func TestChooseCoreReviewers(t *testing.T) {
	if len(AvailableReviewers(nil)) < 2 {
		t.Fatalf("not enough available reviewers (%v) to test (need at least 2)", AvailableReviewers(nil))
	}
	firstCoreReviewer := AvailableReviewers(nil)[0]
	secondCoreReviewer := AvailableReviewers(nil)[1]
	cases := map[string]struct {
		RequestedReviewers                               []User
		PreviousReviewers                                []User
		ExpectReviewersFromList, ExpectSpecificReviewers []string
		ExpectPrimaryReviewer                            bool
	}{
		"no previous review requests assigns new reviewer from team": {
			RequestedReviewers:      []User{},
			PreviousReviewers:       []User{},
			ExpectReviewersFromList: AvailableReviewers(nil),
			ExpectPrimaryReviewer:   true,
		},
		"requested reviewer from team means that primary reviewer was already selected": {
			RequestedReviewers:    []User{User{Login: firstCoreReviewer}},
			PreviousReviewers:     []User{},
			ExpectPrimaryReviewer: false,
		},
		"requested off-team reviewer does not mean that primary reviewer was already selected": {
			RequestedReviewers:    []User{User{Login: "foobar"}},
			PreviousReviewers:     []User{},
			ExpectPrimaryReviewer: true,
		},
		"previously involved team member reviewers should have review requested and mean that primary reviewer was already selected": {
			RequestedReviewers:      []User{},
			PreviousReviewers:       []User{User{Login: firstCoreReviewer}},
			ExpectSpecificReviewers: []string{firstCoreReviewer},
			ExpectPrimaryReviewer:   false,
		},
		"previously involved reviewers that are not team members are ignored": {
			RequestedReviewers:      []User{},
			PreviousReviewers:       []User{User{Login: "foobar"}},
			ExpectReviewersFromList: AvailableReviewers(nil),
			ExpectPrimaryReviewer:   true,
		},
		"only previously involved team member reviewers will have review requested": {
			RequestedReviewers:      []User{},
			PreviousReviewers:       []User{User{Login: firstCoreReviewer}, User{Login: "foobar"}, User{Login: secondCoreReviewer}},
			ExpectSpecificReviewers: []string{firstCoreReviewer, secondCoreReviewer},
			ExpectPrimaryReviewer:   false,
		},
		"primary reviewer will not have review requested even if other team members previously reviewed": {
			RequestedReviewers:      []User{User{Login: secondCoreReviewer}},
			PreviousReviewers:       []User{User{Login: firstCoreReviewer}},
			ExpectSpecificReviewers: []string{firstCoreReviewer},
			ExpectPrimaryReviewer:   false,
		},
	}

	for tn, tc := range cases {
		tc := tc
		t.Run(tn, func(t *testing.T) {
			t.Parallel()
			reviewers, primaryReviewer := ChooseCoreReviewers(tc.RequestedReviewers, tc.PreviousReviewers)
			if tc.ExpectPrimaryReviewer && primaryReviewer == "" {
				t.Error("wanted primary reviewer to be returned; got none")
			}
			if !tc.ExpectPrimaryReviewer && primaryReviewer != "" {
				t.Errorf("wanted no primary reviewer; got %s", primaryReviewer)
			}
			if len(tc.ExpectReviewersFromList) > 0 {
				for _, reviewer := range reviewers {
					if !slices.Contains(tc.ExpectReviewersFromList, reviewer) {
						t.Errorf("wanted reviewer %s to be in list %v but they were not", reviewer, tc.ExpectReviewersFromList)
					}
				}
			}
			if len(tc.ExpectSpecificReviewers) > 0 {
				if !slices.Equal(reviewers, tc.ExpectSpecificReviewers) {
					t.Errorf("wanted reviewers to be %v; instead got %v", tc.ExpectSpecificReviewers, reviewers)
				}
			}
		})
	}
}

func TestFormatReviewerComment(t *testing.T) {
	cases := map[string]struct {
		Reviewer       string
		AuthorUserType UserType
		Trusted        bool
	}{
		"community contributor": {
			Reviewer:       "foobar",
			AuthorUserType: CommunityUserType,
			Trusted:        false,
		},
		"googler": {
			Reviewer:       "foobar",
			AuthorUserType: GooglerUserType,
			Trusted:        true,
		},
		"core contributor": {
			Reviewer:       "foobar",
			AuthorUserType: CoreContributorUserType,
			Trusted:        true,
		},
	}

	for tn, tc := range cases {
		tc := tc
		t.Run(tn, func(t *testing.T) {
			t.Parallel()
			comment := FormatReviewerComment(tc.Reviewer)
			t.Log(comment)
			if !strings.Contains(comment, fmt.Sprintf("@%s", tc.Reviewer)) {
				t.Errorf("wanted comment to contain @%s; does not.", tc.Reviewer)
			}
			if !strings.Contains(comment, "Tests will require approval") {
				t.Errorf("wanted comment to say tests will require approval; does not")
			}
		})

	}
}

func TestFindReviewerComment(t *testing.T) {
	cases := map[string]struct {
		Comments        []PullRequestComment
		ExpectReviewer  string
		ExpectCommentID int
	}{
		"no reviewer comment": {
			Comments: []PullRequestComment{
				{
					Body: "this is not a reviewer comment",
				},
			},
			ExpectReviewer:  "",
			ExpectCommentID: 0,
		},
		"reviewer comment": {
			Comments: []PullRequestComment{
				{
					Body: FormatReviewerComment("trodge"),
					ID:   1234,
				},
			},
			ExpectReviewer:  "trodge",
			ExpectCommentID: 1234,
		},
		"multiple reviewer comments": {
			Comments: []PullRequestComment{
				{
					Body:      FormatReviewerComment("trodge"),
					ID:        1234,
					CreatedAt: time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
				},
				{
					Body:      FormatReviewerComment("c2thorn"),
					ID:        5678,
					CreatedAt: time.Date(2023, 12, 3, 0, 0, 0, 0, time.UTC),
				},
				{
					Body:      FormatReviewerComment("melinath"),
					ID:        91011,
					CreatedAt: time.Date(2023, 12, 2, 0, 0, 0, 0, time.UTC),
				},
			},
			ExpectReviewer:  "c2thorn",
			ExpectCommentID: 5678,
		},
	}

	for tn, tc := range cases {
		tc := tc
		t.Run(tn, func(t *testing.T) {
			t.Parallel()
			comment, reviewer := FindReviewerComment(tc.Comments)
			if reviewer != tc.ExpectReviewer {
				t.Errorf("wanted reviewer to be %s; got %s", tc.ExpectReviewer, reviewer)
			}
			if comment.ID != tc.ExpectCommentID {
				t.Errorf("wanted comment ID to be %d; got %d", tc.ExpectCommentID, comment.ID)
			}
		})
	}
}
