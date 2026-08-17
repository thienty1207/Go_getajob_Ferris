package processor

import (
	"errors"
	"math"
	"strings"

	locationkey "github.com/gogetsomefoodferris/backend/internal/location"
	"github.com/gogetsomefoodferris/backend/internal/model"
)

var ErrInvalidCandidate = errors.New("invalid job candidate")

func ScoreCandidate(profile model.StructuredProfile, candidate model.JobCandidate) (model.ScoredJobMatch, error) {
	if candidate.ID == [16]byte{} || strings.TrimSpace(candidate.Title) == "" || strings.TrimSpace(candidate.Role) == "" {
		return model.ScoredJobMatch{}, ErrInvalidCandidate
	}
	if !isSupportedWorkMode(candidate.WorkMode) {
		return model.ScoredJobMatch{}, ErrInvalidCandidate
	}
	if profile.YearsOfExperience < 0 || profile.YearsOfExperience > 100 {
		return model.ScoredJobMatch{}, ErrInvalidCandidate
	}

	required := weightedOverlap(profile.Skills, candidate.RequiredSkills, 35)
	role := roleRelevance(profile.Roles, candidate.Role, candidate.Title)
	experience := experiencePoints(profile.YearsOfExperience, candidate.MinimumExperience)
	seniority := seniorityPoints(profile.Seniority, candidate.Seniority)
	preferredDomain := preferredDomainPoints(profile, candidate)

	return model.ScoredJobMatch{
		JobID:                       candidate.ID,
		RequiredSkillsPoints:        roundScore(required),
		RoleRelevancePoints:         roundScore(role),
		ExperiencePoints:            roundScore(experience),
		SeniorityPoints:             roundScore(seniority),
		PreferredSkillsDomainPoints: roundScore(preferredDomain),
		MatchPercent:                roundScore(required + role + experience + seniority + preferredDomain),
		DistanceKm:                  candidate.DistanceKm,
	}, nil
}

func weightedOverlap(profileValues, candidateValues []string, weight float64) float64 {
	values := uniqueNormalized(candidateValues)
	if len(values) == 0 {
		return weight
	}
	profileSet := normalizedSet(profileValues)
	matched := 0
	for _, value := range values {
		if _, ok := profileSet[value]; ok {
			matched++
		}
	}
	return weight * float64(matched) / float64(len(values))
}

func roleRelevance(profileRoles []string, candidateRole, candidateTitle string) float64 {
	roleText := normalizedSet(strings.Fields(locationkey.Normalize(candidateRole + " " + candidateTitle)))
	if len(roleText) == 0 {
		return 0
	}
	best := 0.0
	for _, profileRole := range profileRoles {
		profileText := normalizedSet(strings.Fields(locationkey.Normalize(profileRole)))
		if len(profileText) == 0 {
			continue
		}
		matched := 0
		for token := range profileText {
			if _, ok := roleText[token]; ok {
				matched++
			}
		}
		candidate := float64(matched) / float64(maxInt(len(profileText), len(roleText)))
		if strings.Contains(locationkey.Normalize(candidateRole), locationkey.Normalize(profileRole)) || strings.Contains(locationkey.Normalize(profileRole), locationkey.Normalize(candidateRole)) {
			candidate = 1
		}
		if candidate > best {
			best = candidate
		}
	}
	return 25 * math.Min(best, 1)
}

func experiencePoints(years float64, minimum *float64) float64 {
	if minimum == nil || *minimum <= 0 {
		return 15
	}
	return 15 * math.Min(years/(*minimum), 1)
}

func seniorityPoints(profileSeniority, candidateSeniority string) float64 {
	job := strings.ToUpper(strings.TrimSpace(candidateSeniority))
	if job == "" || job == "UNSPECIFIED" {
		return 15
	}
	if strings.ToUpper(strings.TrimSpace(profileSeniority)) == job {
		return 15
	}
	return 0
}

func preferredDomainPoints(profile model.StructuredProfile, candidate model.JobCandidate) float64 {
	targets := uniqueNormalized(append(append([]string{}, candidate.PreferredSkills...), candidate.Domains...))
	if len(targets) == 0 {
		return 10
	}
	profileValues := append(append([]string{}, profile.Skills...), profile.Domains...)
	profileSet := normalizedSet(profileValues)
	matched := 0
	for _, target := range targets {
		if _, ok := profileSet[target]; ok {
			matched++
		}
	}
	return 10 * float64(matched) / float64(len(targets))
}

func normalizedSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized := locationkey.Normalize(value)
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func uniqueNormalized(values []string) []string {
	set := normalizedSet(values)
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	return result
}

func roundScore(value float64) float64 {
	return math.Round(value*100) / 100
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func isSupportedWorkMode(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "REMOTE", "HYBRID", "ONSITE":
		return true
	default:
		return false
	}
}
