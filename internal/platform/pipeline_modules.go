package platform

import "strings"

func BuiltinPipelineModules() []PipelineModule {
	return []PipelineModule{
		pipelineModule("gitlab_checkout", "拉取 GitLab 代码", "source", "git", "从 GitLab 拉取指定分支或提交到工作目录。", `if [ -d "${WORKSPACE}/.git" ]; then git -C "${WORKSPACE}" fetch --all --prune && git -C "${WORKSPACE}" checkout "${GIT_REF}"; else git clone --branch "${GIT_REF}" "${GITLAB_REPO_URL}" "${WORKSPACE}"; fi`, "WORKSPACE", "GIT_REF", "GITLAB_REPO_URL"),
		pipelineModule("go_test", "Go 单元测试", "test", "golang", "执行 Go 单元测试。", `cd "${WORKSPACE}" && go test ./...`, "WORKSPACE"),
		pipelineModule("go_build", "Golang Build", "build", "golang", "编译 Go 服务二进制。", `cd "${WORKSPACE}" && CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" go build -o "${OUTPUT_BIN:-dist/${APP_ID}}" "${GO_BUILD_PACKAGE:-./app}"`, "WORKSPACE", "APP_ID", "CGO_ENABLED", "GOOS", "GOARCH", "OUTPUT_BIN", "GO_BUILD_PACKAGE"),
		pipelineModule("jdk8_maven_build", "JDK8 Maven Build", "build", "jdk8", "使用 JDK8 和 Maven 构建 Java 服务。", `export JAVA_HOME="${JDK8_HOME}" PATH="${JAVA_HOME}/bin:${PATH}" && cd "${WORKSPACE}" && mvn -B -DskipTests="${MAVEN_SKIP_TESTS:-true}" package`, "WORKSPACE", "JDK8_HOME", "MAVEN_SKIP_TESTS"),
		pipelineModule("python_install", "Python 依赖安装", "build", "python", "安装 Python 项目依赖。", `cd "${WORKSPACE}" && python -m pip install -r "${PYTHON_REQUIREMENTS:-requirements.txt}"`, "WORKSPACE", "PYTHON_REQUIREMENTS"),
		pipelineModule("python_test", "Python 单元测试", "test", "python", "执行 Python 单元测试。", `cd "${WORKSPACE}" && if [ -f pytest.ini ] || [ -d tests ]; then python -m pytest; else echo "skip pytest: no pytest.ini or tests directory"; fi`, "WORKSPACE"),
		pipelineModule("c_make_build", "C Make Build", "build", "c", "使用 Make 构建 C 项目。", `cd "${WORKSPACE}" && make ${MAKE_TARGET:-build}`, "WORKSPACE", "MAKE_TARGET"),
		pipelineModule("c_test", "C 测试", "test", "c", "执行 C 项目测试目标。", `cd "${WORKSPACE}" && make ${TEST_TARGET:-test}`, "WORKSPACE", "TEST_TARGET"),
		pipelineModule("cpp_cmake_build", "C++ CMake Build", "build", "cpp", "使用 CMake 构建 C++ 项目。", `cd "${WORKSPACE}" && cmake -S . -B "${BUILD_DIR:-build}" -DCMAKE_BUILD_TYPE="${CMAKE_BUILD_TYPE:-Release}" && cmake --build "${BUILD_DIR:-build}" --parallel`, "WORKSPACE", "BUILD_DIR", "CMAKE_BUILD_TYPE"),
		pipelineModule("cpp_test", "C++ 测试", "test", "cpp", "执行 CTest。", `cd "${WORKSPACE}" && ctest --test-dir "${BUILD_DIR:-build}" --output-on-failure`, "WORKSPACE", "BUILD_DIR"),
		pipelineModule("dotnet_restore", ".NET Restore", "build", "dotnet", "恢复 .NET 依赖。", `cd "${WORKSPACE}" && dotnet restore`, "WORKSPACE"),
		pipelineModule("dotnet_publish", ".NET Publish", "build", "dotnet", "发布 .NET 服务产物。", `cd "${WORKSPACE}" && dotnet publish "${DOTNET_PROJECT:-.}" -c "${DOTNET_CONFIGURATION:-Release}" -o "${OUTPUT_DIR:-publish}"`, "WORKSPACE", "DOTNET_PROJECT", "DOTNET_CONFIGURATION", "OUTPUT_DIR"),
		pipelineModule("dotnet_test", ".NET Test", "test", "dotnet", "执行 .NET 测试。", `cd "${WORKSPACE}" && dotnet test --no-restore`, "WORKSPACE"),
		pipelineModule("rust_test", "Rust Test", "test", "rust", "执行 Rust 测试。", `cd "${WORKSPACE}" && cargo test --locked`, "WORKSPACE"),
		pipelineModule("rust_build", "Rust Build", "build", "rust", "构建 Rust release 产物。", `cd "${WORKSPACE}" && cargo build --release --locked`, "WORKSPACE"),
		pipelineModule("php_composer_install", "PHP Composer Install", "build", "php", "安装 PHP Composer 依赖。", `cd "${WORKSPACE}" && composer install --no-interaction --prefer-dist --optimize-autoloader`, "WORKSPACE"),
		pipelineModule("php_test", "PHP 单元测试", "test", "php", "执行 PHP 单元测试，仓库没有 phpunit 时自动跳过。", `cd "${WORKSPACE}" && if [ -x vendor/bin/phpunit ]; then vendor/bin/phpunit; else echo "skip phpunit: vendor/bin/phpunit not found"; fi`, "WORKSPACE"),
		pipelineModule("node_install", "Node 依赖安装", "build", "node", "安装前端依赖。", `cd "${WORKSPACE}" && ${PACKAGE_MANAGER:-pnpm} install --frozen-lockfile`, "WORKSPACE", "PACKAGE_MANAGER"),
		pipelineModule("npm_lint", "前端 Lint", "test", "node", "执行前端静态检查。", `cd "${WORKSPACE}" && ${PACKAGE_MANAGER:-pnpm} lint`, "WORKSPACE", "PACKAGE_MANAGER"),
		pipelineModule("npm_build", "前端 Build", "build", "node", "构建前端产物。", `cd "${WORKSPACE}" && ${PACKAGE_MANAGER:-pnpm} build`, "WORKSPACE", "PACKAGE_MANAGER"),
		pipelineModule("static_artifact", "静态产物打包", "artifact", "node", "打包前端静态产物。", `cd "${WORKSPACE}" && tar -czf "dist/static-${VERSION}.tgz" dist`, "WORKSPACE", "VERSION"),
		pipelineModule("docker_build_push", "Docker Build & Push", "image", "docker", "构建并推送镜像到镜像仓库。", `IMAGE="${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_ID}:${DEPLOY_ENV}_v${BUILD_NUMBER}" && docker build -t "${IMAGE}" "${WORKSPACE}" && docker push "${IMAGE}"`, "HARBOR_REGISTRY", "HARBOR_PROJECT", "APP_ID", "DEPLOY_ENV", "BUILD_NUMBER", "WORKSPACE"),
		pipelineModule("nginx_image", "Nginx 静态镜像", "image", "docker", "使用 Nginx Dockerfile 构建前端镜像。", `IMAGE="${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_ID}:${DEPLOY_ENV}_v${BUILD_NUMBER}" && docker build -f "${WORKSPACE}/Dockerfile.nginx" -t "${IMAGE}" "${WORKSPACE}" && docker push "${IMAGE}"`, "HARBOR_REGISTRY", "HARBOR_PROJECT", "APP_ID", "DEPLOY_ENV", "BUILD_NUMBER", "WORKSPACE"),
		pipelineModule("k8s_manifest_image", "更新 K8s Manifest 镜像", "deploy", "k8s", "把 manifest 中的镜像版本替换为当前构建号。", `sed -i -r "/image: ${HARBOR_REGISTRY}/ s/(_v[0-9]+)/_v${BUILD_NUMBER}/g" "${K8S_MANIFEST}"`, "HARBOR_REGISTRY", "BUILD_NUMBER", "K8S_MANIFEST"),
		pipelineModule("k8s_canary_first", "K8s 灰度第一批", "deploy", "k8s", "应用 manifest 后暂停 rollout，只更新第一批副本。", `kubectl apply -f "${K8S_MANIFEST}" && kubectl rollout pause deployment "${APP_ID}" -n "${DEPLOY_ENV}"`, "K8S_MANIFEST", "APP_ID", "DEPLOY_ENV"),
		pipelineModule("k8s_canary_rest", "K8s 灰度剩余副本", "deploy", "k8s", "恢复 rollout，继续更新剩余副本。", `kubectl rollout resume deployment "${APP_ID}" -n "${DEPLOY_ENV}"`, "APP_ID", "DEPLOY_ENV"),
		pipelineModule("k8s_deploy_all", "K8s 全量发布", "deploy", "k8s", "一次性应用 manifest 并发布全部副本。", `kubectl apply -f "${K8S_MANIFEST}"`, "K8S_MANIFEST"),
		pipelineModule("k8s_restart", "K8s 重启 Deployment", "deploy", "k8s", "通过 patch 环境变量触发 Deployment 重启。", `kubectl patch deployment "${APP_ID}" -n "${DEPLOY_ENV}" -p '{"spec":{"template":{"spec":{"containers":[{"name":"'"${APP_ID}"'","env":[{"name":"RESTART_","value":"'"$(date +%s)"'"}]}]}}}}'`, "APP_ID", "DEPLOY_ENV"),
		pipelineModule("k8s_rollback", "K8s 回滚版本", "deploy", "k8s", "把 manifest 镜像替换到回滚版本并重新应用。", `sed -i -r "/image: ${HARBOR_REGISTRY}/ s/(v[0-9]+)/v${ROLLBACK_VERSION}/g" "${K8S_MANIFEST}" && kubectl apply -f "${K8S_MANIFEST}"`, "HARBOR_REGISTRY", "ROLLBACK_VERSION", "K8S_MANIFEST"),
		pipelineModule("k8s_pod_status", "查看 Pod 状态", "verify", "k8s", "周期性查看应用 Pod 状态。", `for i in $(seq "${POD_CHECK_TIMES:-7}"); do sleep "${POD_CHECK_INTERVAL:-3}"; kubectl get pod -n "${DEPLOY_ENV}" -l "app=${APP_ID}"; done`, "POD_CHECK_TIMES", "POD_CHECK_INTERVAL", "DEPLOY_ENV", "APP_ID"),
		pipelineModule("k8s_canary_check", "检查灰度 ReplicaSet", "verify", "k8s", "确认灰度阶段存在两个 ReplicaSet。", `rs_num=$(kubectl get pod -n "${DEPLOY_ENV}" | grep "${APP_ID}" | grep Running | awk -F"-" '{print $(NF-1)}' | cut -d " " -f 1 | sort -u | wc -l); test "${rs_num}" -eq 2`, "DEPLOY_ENV", "APP_ID"),
		pipelineModule("k8s_rollout_status", "检查 Rollout 状态", "verify", "k8s", "等待 Deployment rollout 成功。", `kubectl rollout status deployment/"${APP_ID}" -n "${DEPLOY_ENV}" | grep successful`, "APP_ID", "DEPLOY_ENV"),
		pipelineModule("notification", "发布通知", "notify", "webhook", "通过 Webhook 发送流水线结果。", `curl -X POST "${WEBHOOK_URL}" -d "text=${APP_ID} ${DEPLOY_ENV}_v${BUILD_NUMBER} ${DEPLOY_ACTION} completed"`, "WEBHOOK_URL", "APP_ID", "DEPLOY_ENV", "BUILD_NUMBER", "DEPLOY_ACTION"),
	}
}

func pipelineModule(key, name, category, runtime, description, command string, variables ...string) PipelineModule {
	return PipelineModule{
		Key:         key,
		Name:        name,
		Category:    category,
		Runtime:     runtime,
		Description: description,
		Command:     command,
		Variables:   variables,
		Status:      "enabled",
	}
}

func (s *Store) ensureBuiltinPipelineModulesLocked() bool {
	changed := false
	existing := make(map[string]bool, len(s.PipelineModules))
	for _, module := range s.PipelineModules {
		existing[strings.TrimSpace(module.Key)] = true
	}
	for _, module := range BuiltinPipelineModules() {
		if existing[module.Key] {
			continue
		}
		module.ID = s.Next("pipeline_module")
		s.PipelineModules = append(s.PipelineModules, module)
		existing[module.Key] = true
		changed = true
	}
	return changed
}

func (s *Store) PipelineModuleByKey(key string) (PipelineModule, bool) {
	key = strings.TrimSpace(key)
	for _, module := range s.PipelineModules {
		if module.Key == key {
			return module, true
		}
	}
	return PipelineModule{}, false
}

func DefaultPipelineModuleKey(stepType string) string {
	switch strings.TrimSpace(stepType) {
	case "git":
		return "gitlab_checkout"
	case "docker_build":
		return "docker_build_push"
	case "k8s_deploy":
		return "k8s_deploy_all"
	default:
		return strings.TrimSpace(stepType)
	}
}

func ModulePipelineStep(name, moduleKey string) PipelineStep {
	moduleKey = strings.TrimSpace(moduleKey)
	return PipelineStep{Name: name, Type: moduleKey, ModuleKey: moduleKey}
}
