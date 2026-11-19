package identity

import "context"

type ProfileExtras struct {
	EnrolledCourses []string `json:"enrolledCourses,omitzero"`
	Track           *string  `json:"track,omitempty"`
	Courses         []string `json:"courses,omitzero"`
	Tracks          []string `json:"tracks,omitzero"`
}

type ProfileExtender interface {
	ProfileExtras(ctx context.Context, userID string, roles []string) (ProfileExtras, error)
}

type UserScoper interface {
	VisibleUserIDs(ctx context.Context, viewerID string, roles []string) ([]string, error)
}

type ProfileResponse struct {
	UserResponse
	ProfileExtras
}

func (s *Service) SetProfileExtender(e ProfileExtender) {
	s.extender = e
}

func (s *Service) SetUserScoper(sc UserScoper) {
	s.scoper = sc
}

func (s *Service) Profile(ctx context.Context, u *User) (ProfileResponse, error) {
	resp := ProfileResponse{UserResponse: ToUserResponse(u)}
	if s.extender == nil {
		return resp, nil
	}
	extras, err := s.extender.ProfileExtras(ctx, u.ID, u.Roles)
	if err != nil {
		return resp, err
	}
	resp.ProfileExtras = extras
	return resp, nil
}
