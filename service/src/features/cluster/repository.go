package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"ivory/src/clients/database"
	"ivory/src/clients/sidecar"
	"ivory/src/features/cert"
	"ivory/src/storage/db"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Repository struct {
	bucket        *db.Bucket[Cluster]
	filePath      string
	directoryMode bool
	mutex         *sync.Mutex
}

type fileCredentials struct {
	PatroniId  *uuid.UUID            `json:"patroniId,omitempty"`
	PostgresId *uuid.UUID            `json:"postgresId,omitempty"`
	Patroni    *database.Credentials `json:"patroni,omitempty"`
	Postgres   *database.Credentials `json:"postgres,omitempty"`
}

type fileCluster struct {
	Name        string            `json:"name"`
	Sidecars    []sidecar.Sidecar `json:"sidecars"`
	Tls         ClusterTls        `json:"tls"`
	Certs       cert.Certs        `json:"certs"`
	Credentials fileCredentials   `json:"credentials"`
	Tags        []string          `json:"tags"`
}

type fileContainer struct {
	Clusters []fileCluster `json:"clusters"`
}

type sourceShape uint8

const (
	sourceShapeSingle sourceShape = iota
	sourceShapeList
	sourceShapeContainer
)

type configSource struct {
	Path     string
	Shape    sourceShape
	Clusters []fileCluster
}

var fileNameInvalidCharRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func NewRepository(bucket *db.Bucket[Cluster]) *Repository {
	return &Repository{
		bucket: bucket,
	}
}

func NewFileRepository(bucket *db.Bucket[Cluster], filePath string) *Repository {
	repository := &Repository{
		bucket:        bucket,
		filePath:      filePath,
		directoryMode: detectDirectoryMode(filePath),
		mutex:         &sync.Mutex{},
	}
	if err := repository.ensureFile(); err != nil {
		panic(err)
	}
	if err := repository.migrateFromLegacySingleFileIfEmpty(); err != nil {
		panic(err)
	}
	if err := repository.migrateFromDbIfEmpty(); err != nil {
		panic(err)
	}
	return repository
}

func (r *Repository) List() ([]Cluster, error) {
	if !r.isFileMode() {
		return r.bucket.GetList(nil, nil)
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, err
	}
	result := make([]Cluster, 0, len(clusterFile))
	for _, c := range clusterFile {
		result = append(result, r.toCluster(c))
	}
	return result, nil
}

func (r *Repository) ListByName(clusters []string) ([]Cluster, error) {
	if !r.isFileMode() {
		clusterMap := make(map[string]bool)
		for _, c := range clusters {
			clusterMap[c] = true
		}
		return r.bucket.GetList(func(cert Cluster) bool {
			return clusterMap[cert.Name]
		}, nil)
	}
	clusterMap := make(map[string]bool)
	for _, c := range clusters {
		clusterMap[c] = true
	}
	list, err := r.List()
	if err != nil {
		return nil, err
	}
	filtered := make([]Cluster, 0)
	for _, c := range list {
		if clusterMap[c.Name] {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func (r *Repository) Get(key string) (Cluster, error) {
	if !r.isFileMode() {
		return r.bucket.Get(key)
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return Cluster{}, err
	}
	for _, c := range clusterFile {
		if c.Name == key {
			return r.toCluster(c), nil
		}
	}
	return Cluster{}, db.ErrNotFound
}

func (r *Repository) Update(cluster Cluster) error {
	if !r.isFileMode() {
		return r.bucket.Update(cluster.Name, cluster)
	}
	if cluster.Name == "" {
		return db.ErrEmptyKey
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.directoryMode {
		return r.updateDirectoryCluster(cluster)
	}

	clusterFile, err := r.readFile()
	if err != nil {
		return err
	}

	index := -1
	for i, element := range clusterFile.Clusters {
		if element.Name == cluster.Name {
			index = i
			break
		}
	}

	if index >= 0 {
		current := clusterFile.Clusters[index]
		clusterFile.Clusters[index] = r.toFileCluster(cluster, &current)
	} else {
		clusterFile.Clusters = append(clusterFile.Clusters, r.toFileCluster(cluster, nil))
	}

	return r.writeFile(clusterFile)
}

func (r *Repository) Create(cluster Cluster) (Cluster, error) {
	if !r.isFileMode() {
		return r.bucket.Create(cluster.Name, cluster)
	}
	if cluster.Name == "" {
		return cluster, db.ErrEmptyKey
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.directoryMode {
		return r.createDirectoryCluster(cluster)
	}

	clusterFile, err := r.readFile()
	if err != nil {
		return cluster, err
	}
	for _, element := range clusterFile.Clusters {
		if element.Name == cluster.Name {
			return cluster, db.ErrAlreadyExists
		}
	}

	clusterFile.Clusters = append(clusterFile.Clusters, r.toFileCluster(cluster, nil))
	return cluster, r.writeFile(clusterFile)
}

func (r *Repository) Delete(key string) error {
	if !r.isFileMode() {
		return r.bucket.Delete(key)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.directoryMode {
		return r.deleteDirectoryCluster(key)
	}

	clusterFile, err := r.readFile()
	if err != nil {
		return err
	}
	filtered := make([]fileCluster, 0, len(clusterFile.Clusters))
	for _, cluster := range clusterFile.Clusters {
		if cluster.Name != key {
			filtered = append(filtered, cluster)
		}
	}
	clusterFile.Clusters = filtered
	return r.writeFile(clusterFile)
}

func (r *Repository) DeleteAll() error {
	if !r.isFileMode() {
		return r.bucket.DeleteAll()
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.directoryMode {
		return r.deleteAllDirectoryClusters()
	}

	return r.writeFile(fileContainer{Clusters: []fileCluster{}})
}

func (r *Repository) ResolvePatroniBySidecar(s sidecar.Sidecar) (*sidecar.Credentials, bool) {
	if !r.isFileMode() {
		return nil, false
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, false
	}
	for _, cluster := range clusterFile {
		for _, clusterSidecar := range cluster.Sidecars {
			if equalSidecar(clusterSidecar, s) {
				return toSidecarCredentials(cluster.Credentials.Patroni)
			}
		}
	}
	return nil, false
}

func (r *Repository) ResolvePatroniByCluster(clusterName string) (*sidecar.Credentials, bool) {
	if !r.isFileMode() {
		return nil, false
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, false
	}
	for _, cluster := range clusterFile {
		if cluster.Name == clusterName {
			return toSidecarCredentials(cluster.Credentials.Patroni)
		}
	}
	return nil, false
}

func (r *Repository) ResolvePostgresByDatabase(dbInfo database.Database) (*database.Credentials, bool) {
	if !r.isFileMode() {
		return nil, false
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, false
	}

	var singleHostMatch *database.Credentials
	hostMatchCount := 0
	for _, cluster := range clusterFile {
		exactMatch := false
		hostMatch := false
		for _, clusterSidecar := range cluster.Sidecars {
			if strings.EqualFold(clusterSidecar.Host, dbInfo.Host) {
				hostMatch = true
				if clusterSidecar.Port == dbInfo.Port {
					exactMatch = true
					break
				}
			}
		}
		if exactMatch {
			return toDatabaseCredentials(cluster.Credentials.Postgres)
		}
		if hostMatch {
			credentials, ok := toDatabaseCredentials(cluster.Credentials.Postgres)
			if ok {
				singleHostMatch = credentials
				hostMatchCount++
			}
		}
	}
	if hostMatchCount == 1 {
		return singleHostMatch, true
	}
	return nil, false
}

func (r *Repository) ResolvePostgresBySidecarAddress(sidecarInfo database.SidecarAddress) (*database.Credentials, bool) {
	if !r.isFileMode() {
		return nil, false
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, false
	}
	for _, cluster := range clusterFile {
		for _, clusterSidecar := range cluster.Sidecars {
			if strings.EqualFold(clusterSidecar.Host, sidecarInfo.Host) && clusterSidecar.Port == sidecarInfo.Port {
				return toDatabaseCredentials(cluster.Credentials.Postgres)
			}
		}
	}
	return nil, false
}

func (r *Repository) ResolvePostgresByCluster(clusterName string) (*database.Credentials, bool) {
	if !r.isFileMode() {
		return nil, false
	}
	clusterFile, err := r.readClusters()
	if err != nil {
		return nil, false
	}
	for _, cluster := range clusterFile {
		if cluster.Name == clusterName {
			return toDatabaseCredentials(cluster.Credentials.Postgres)
		}
	}
	return nil, false
}

func (r *Repository) isFileMode() bool {
	return r.filePath != ""
}

func detectDirectoryMode(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir()
	}
	return strings.ToLower(filepath.Ext(path)) != ".json"
}

func (r *Repository) ensureFile() error {
	if !r.isFileMode() {
		return nil
	}

	if r.directoryMode {
		info, err := os.Stat(r.filePath)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("clusters path %s should be a directory", r.filePath)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.MkdirAll(r.filePath, os.ModePerm)
	}

	dir := filepath.Dir(r.filePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	if _, err := os.Stat(r.filePath); errors.Is(err, os.ErrNotExist) {
		return r.writeFile(fileContainer{Clusters: []fileCluster{}})
	}
	return nil
}

func (r *Repository) migrateFromLegacySingleFileIfEmpty() error {
	if !r.directoryMode {
		return nil
	}
	list, err := r.readDirectoryClusters()
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return nil
	}

	legacyPath := r.filePath + ".json"
	if _, err := os.Stat(legacyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	legacySource, err := r.readConfigSource(legacyPath)
	if err != nil {
		return err
	}
	if len(legacySource.Clusters) == 0 {
		return nil
	}

	used := make(map[string]bool)
	for _, cl := range legacySource.Clusters {
		if cl.Name == "" {
			continue
		}
		path := r.newDirectorySourcePath(cl.Name, used)
		used[strings.ToLower(path)] = true
		errWrite := r.writeConfigSource(configSource{
			Path:     path,
			Shape:    sourceShapeSingle,
			Clusters: []fileCluster{cl},
		})
		if errWrite != nil {
			return errWrite
		}
	}
	return nil
}

func (r *Repository) migrateFromDbIfEmpty() error {
	if !r.isFileMode() || r.bucket == nil {
		return nil
	}

	if r.directoryMode {
		clusterList, err := r.readDirectoryClusters()
		if err != nil {
			return err
		}
		if len(clusterList) > 0 {
			return nil
		}
	} else {
		clusterFile, err := r.readFile()
		if err != nil {
			return err
		}
		if len(clusterFile.Clusters) > 0 {
			return nil
		}
	}

	list, err := r.bucket.GetList(nil, nil)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}

	if r.directoryMode {
		used := make(map[string]bool)
		for _, element := range list {
			path := r.newDirectorySourcePath(element.Name, used)
			used[strings.ToLower(path)] = true
			errWrite := r.writeConfigSource(configSource{
				Path:     path,
				Shape:    sourceShapeSingle,
				Clusters: []fileCluster{r.toFileCluster(element, nil)},
			})
			if errWrite != nil {
				return errWrite
			}
		}
		return nil
	}

	clusterFile := fileContainer{Clusters: make([]fileCluster, 0, len(list))}
	for _, element := range list {
		clusterFile.Clusters = append(clusterFile.Clusters, r.toFileCluster(element, nil))
	}
	return r.writeFile(clusterFile)
}

func (r *Repository) readClusters() ([]fileCluster, error) {
	if r.directoryMode {
		return r.readDirectoryClusters()
	}
	clusterFile, err := r.readFile()
	if err != nil {
		return nil, err
	}
	return clusterFile.Clusters, nil
}

func (r *Repository) readDirectoryClusters() ([]fileCluster, error) {
	sources, err := r.readDirectorySources()
	if err != nil {
		return nil, err
	}
	result := make([]fileCluster, 0)
	seen := make(map[string]bool)
	for _, src := range sources {
		for _, cl := range src.Clusters {
			if cl.Name == "" {
				continue
			}
			if seen[cl.Name] {
				continue
			}
			seen[cl.Name] = true
			result = append(result, cl)
		}
	}
	return result, nil
}

func (r *Repository) readDirectorySources() ([]configSource, error) {
	list, err := os.ReadDir(r.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []configSource{}, nil
		}
		return nil, err
	}

	sources := make([]configSource, 0)
	for _, element := range list {
		if element.IsDir() {
			continue
		}
		if !isClusterConfigFile(element.Name()) {
			continue
		}
		path := filepath.Join(r.filePath, element.Name())
		source, errSource := r.readConfigSource(path)
		if errSource != nil {
			return nil, errSource
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func (r *Repository) readConfigSource(path string) (configSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return configSource{}, err
	}
	clusters, shape, err := parseClustersData(data)
	if err != nil {
		return configSource{}, fmt.Errorf("invalid clusters config format in %s: %w", path, err)
	}
	return configSource{
		Path:     path,
		Shape:    shape,
		Clusters: clusters,
	}, nil
}

func (r *Repository) readFile() (fileContainer, error) {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return fileContainer{}, err
	}
	clusters, _, err := parseClustersData(data)
	if err != nil {
		return fileContainer{}, err
	}
	return fileContainer{Clusters: clusters}, nil
}

func parseClustersData(data []byte) ([]fileCluster, sourceShape, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return []fileCluster{}, sourceShapeContainer, nil
	}

	var container fileContainer
	if err := json.Unmarshal(data, &container); err == nil && container.Clusters != nil {
		return container.Clusters, sourceShapeContainer, nil
	}

	var list []fileCluster
	if err := json.Unmarshal(data, &list); err == nil {
		return list, sourceShapeList, nil
	}

	var single fileCluster
	if err := json.Unmarshal(data, &single); err == nil && single.Name != "" {
		return []fileCluster{single}, sourceShapeSingle, nil
	}

	return nil, sourceShapeContainer, errors.New("supported formats: {\"clusters\": [...]}, [...], or single cluster object")
}

func (r *Repository) writeFile(container fileContainer) error {
	if container.Clusters == nil {
		container.Clusters = []fileCluster{}
	}
	data, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, data, 0644)
}

func (r *Repository) updateDirectoryCluster(cluster Cluster) error {
	sources, err := r.readDirectorySources()
	if err != nil {
		return err
	}
	sourceIndex, clusterIndex, found := findClusterInSources(cluster.Name, sources)
	if found {
		current := sources[sourceIndex].Clusters[clusterIndex]
		sources[sourceIndex].Clusters[clusterIndex] = r.toFileCluster(cluster, &current)
		return r.writeConfigSource(sources[sourceIndex])
	}

	used := make(map[string]bool)
	for _, source := range sources {
		used[strings.ToLower(source.Path)] = true
	}
	path := r.newDirectorySourcePath(cluster.Name, used)
	source := configSource{
		Path:     path,
		Shape:    sourceShapeSingle,
		Clusters: []fileCluster{r.toFileCluster(cluster, nil)},
	}
	return r.writeConfigSource(source)
}

func (r *Repository) createDirectoryCluster(cluster Cluster) (Cluster, error) {
	sources, err := r.readDirectorySources()
	if err != nil {
		return cluster, err
	}
	_, _, found := findClusterInSources(cluster.Name, sources)
	if found {
		return cluster, db.ErrAlreadyExists
	}

	used := make(map[string]bool)
	for _, source := range sources {
		used[strings.ToLower(source.Path)] = true
	}
	path := r.newDirectorySourcePath(cluster.Name, used)
	source := configSource{
		Path:     path,
		Shape:    sourceShapeSingle,
		Clusters: []fileCluster{r.toFileCluster(cluster, nil)},
	}
	return cluster, r.writeConfigSource(source)
}

func (r *Repository) deleteDirectoryCluster(name string) error {
	sources, err := r.readDirectorySources()
	if err != nil {
		return err
	}
	sourceIndex, clusterIndex, found := findClusterInSources(name, sources)
	if !found {
		return nil
	}

	source := sources[sourceIndex]
	source.Clusters = append(source.Clusters[:clusterIndex], source.Clusters[clusterIndex+1:]...)
	if len(source.Clusters) == 0 {
		errRem := os.Remove(source.Path)
		if errors.Is(errRem, os.ErrNotExist) {
			return nil
		}
		return errRem
	}
	return r.writeConfigSource(source)
}

func (r *Repository) deleteAllDirectoryClusters() error {
	list, err := os.ReadDir(r.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var res error
	for _, element := range list {
		if element.IsDir() || !isClusterConfigFile(element.Name()) {
			continue
		}
		path := filepath.Join(r.filePath, element.Name())
		errRem := os.Remove(path)
		if errRem != nil && !errors.Is(errRem, os.ErrNotExist) {
			res = errors.Join(res, errRem)
		}
	}
	return res
}

func (r *Repository) writeConfigSource(source configSource) error {
	if source.Clusters == nil {
		source.Clusters = []fileCluster{}
	}
	if source.Shape == sourceShapeSingle && len(source.Clusters) != 1 {
		source.Shape = sourceShapeList
	}

	var body any
	switch source.Shape {
	case sourceShapeSingle:
		body = source.Clusters[0]
	case sourceShapeList:
		body = source.Clusters
	default:
		body = fileContainer{Clusters: source.Clusters}
	}

	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(source.Path, data, 0644)
}

func isClusterConfigFile(name string) bool {
	low := strings.ToLower(name)
	return strings.HasSuffix(low, "-cluster.json") || low == "clusters.json"
}

func findClusterInSources(name string, sources []configSource) (int, int, bool) {
	for sourceIndex, source := range sources {
		for clusterIndex, cluster := range source.Clusters {
			if cluster.Name == name {
				return sourceIndex, clusterIndex, true
			}
		}
	}
	return -1, -1, false
}

func (r *Repository) newDirectorySourcePath(clusterName string, used map[string]bool) string {
	safeName := sanitizeClusterFileName(clusterName)
	base := safeName + "-cluster"

	attempt := 0
	for {
		name := base + ".json"
		if attempt > 0 {
			name = fmt.Sprintf("%s-%d.json", base, attempt+1)
		}
		path := filepath.Join(r.filePath, name)
		lowPath := strings.ToLower(path)
		if used[lowPath] {
			attempt++
			continue
		}
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			return path
		}
		attempt++
	}
}

func sanitizeClusterFileName(name string) string {
	trimmed := strings.TrimSpace(strings.ToLower(name))
	if trimmed == "" {
		return "cluster"
	}
	cleaned := strings.ReplaceAll(trimmed, " ", "-")
	cleaned = fileNameInvalidCharRegex.ReplaceAllString(cleaned, "-")
	cleaned = strings.Trim(cleaned, "-_")
	if cleaned == "" {
		return "cluster"
	}
	return cleaned
}

func (r *Repository) toCluster(cluster fileCluster) Cluster {
	patroniConfigured := cluster.Credentials.PatroniId != nil || hasCredentials(cluster.Credentials.Patroni)
	postgresConfigured := cluster.Credentials.PostgresId != nil || hasCredentials(cluster.Credentials.Postgres)
	return Cluster{
		Name:     cluster.Name,
		Sidecars: cluster.Sidecars,
		ClusterOptions: ClusterOptions{
			Tls:   cluster.Tls,
			Certs: cluster.Certs,
			Credentials: Credentials{
				PatroniId:          cluster.Credentials.PatroniId,
				PostgresId:         cluster.Credentials.PostgresId,
				PatroniConfigured:  patroniConfigured,
				PostgresConfigured: postgresConfigured,
			},
			Tags: cluster.Tags,
		},
	}
}

func (r *Repository) toFileCluster(cluster Cluster, previous *fileCluster) fileCluster {
	credentials := fileCredentials{
		PatroniId:  cluster.Credentials.PatroniId,
		PostgresId: cluster.Credentials.PostgresId,
	}
	if previous != nil {
		if credentials.PatroniId == nil {
			credentials.PatroniId = previous.Credentials.PatroniId
		}
		if credentials.PostgresId == nil {
			credentials.PostgresId = previous.Credentials.PostgresId
		}
		credentials.Patroni = previous.Credentials.Patroni
		credentials.Postgres = previous.Credentials.Postgres
	}

	return fileCluster{
		Name:        cluster.Name,
		Sidecars:    cluster.Sidecars,
		Tls:         cluster.Tls,
		Certs:       cluster.Certs,
		Credentials: credentials,
		Tags:        cluster.Tags,
	}
}

func equalSidecar(a sidecar.Sidecar, b sidecar.Sidecar) bool {
	return strings.EqualFold(a.Host, b.Host) && a.Port == b.Port
}

func hasCredentials(credentials *database.Credentials) bool {
	return credentials != nil && credentials.Username != "" && credentials.Password != ""
}

func toSidecarCredentials(credentials *database.Credentials) (*sidecar.Credentials, bool) {
	if !hasCredentials(credentials) {
		return nil, false
	}
	return &sidecar.Credentials{
		Username: credentials.Username,
		Password: credentials.Password,
	}, true
}

func toDatabaseCredentials(credentials *database.Credentials) (*database.Credentials, bool) {
	if !hasCredentials(credentials) {
		return nil, false
	}
	return &database.Credentials{
		Username: credentials.Username,
		Password: credentials.Password,
	}, true
}
