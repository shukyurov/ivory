package tag

import (
	"strings"
)

type TagsProvider interface {
	ListTags() ([]string, error)
}

type Service struct {
	tagRepository *Repository
	tagsProvider  TagsProvider
}

func NewService(tagRepository *Repository, tagsProvider TagsProvider) *Service {
	return &Service{
		tagRepository: tagRepository,
		tagsProvider:  tagsProvider,
	}
}

func (s *Service) Get(tag string) ([]string, error) {
	return s.tagRepository.Get(tag)
}

func (s *Service) GetMap() (map[string][]string, error) {
	return s.tagRepository.GetMap()
}

func (s *Service) List() ([]string, error) {
	if s.tagsProvider != nil {
		tags, err := s.tagsProvider.ListTags()
		if err == nil {
			return tags, nil
		}
	}
	return s.tagRepository.List()
}

func (s *Service) UpdateCluster(cluster string, tags []string) ([]string, error) {
	var normalizedTags []string
	for _, tag := range tags {
		normalizedTag := strings.TrimSpace(tag)
		if normalizedTag == "" {
			continue
		}
		normalizedTags = append(normalizedTags, normalizedTag)
	}

	tagMap, err := s.tagRepository.GetMap()
	if err != nil {
		return nil, err
	}

	// NOTE: remove cluster from all tags
	for k, v := range tagMap {
		tmp := make([]string, 0)
		for _, c := range v {
			if c != cluster {
				tmp = append(tmp, c)
			}
		}

		if len(tmp) == 0 {
			tagMap[k] = nil
		} else {
			tagMap[k] = tmp
		}
	}

	// NOTE: add cluster to tags
	for _, v := range normalizedTags {
		tagMap[v] = append(tagMap[v], cluster)
	}

	// NOTE: update tags
	for k, v := range tagMap {
		if v != nil {
			err := s.tagRepository.Update(k, v)
			if err != nil {
				return nil, err
			}
		} else {
			err := s.tagRepository.Delete(k)
			if err != nil {
				return nil, err
			}
		}
	}

	return normalizedTags, nil
}

func (s *Service) Delete(tag string) error {
	return s.tagRepository.Delete(tag)
}

func (s *Service) DeleteAll() error {
	return s.tagRepository.DeleteAll()
}
