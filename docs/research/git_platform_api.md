# 1. Способы идентификации платформы по URL репозитория

На данный момент приложение RepoPulse будет иметь доступ только к двум REST API: GitHub и GitLab. 

То есть для того, чтобы идентифицировать платформу, нужно из URL вытащить ключевое слово github.com или gitlab.com, при этом сделав предварительную валидацию, что URL верно построен. 

Пользователь может вставить не только распространенные варианты   ссылок, такие как:
- https://github.com/golang/go
- https://github.com/golang/go.git
- https://gitlab.com/gitlab-org/gitlab
Но и такие варианты:
- git@github.com:golang/go.git
- ssh://git@github.com/golang/go.git
- git@gitlab.com:gitlab-org/gitlab.git
>[!warning]
>И их уже просто вставить в REST API запрос не получиться.


Для этого в приложение нужно реализовать работу с форматом обычного URL и SSH URL. 
Для SSH URL нужно будет реализовать парсер, который будет вытаскивать нужную информацию из SSH URL (owner, repo и тд) и уже по этим данным создавать REST API запрос.

# 2. Получение метаданных публичного репозитория через API GitLab и GitHub

## GitHub. Правила обращения
### Путь
Для того, чтобы получить какие-либо метаданные репозитория через GitHub API нужно нужен путь к конкретному endpoint. Путь обязательно должен состоять из следующих параметров:
`{owner}` - владелец репозитория
`{repo}` - название репозитория
Также обращение к GitHub Api должно быть исключительно по HTTPS.
>[!example]
>То есть весь путь будет выглядить так:
>https://api.github.com/repos/{owner}/{repo}/ 

### HTTP Method
Любой API Request должен в себе содержать HTTP Method:
- `GET`: Используется для получения ресурсов
- `POST`: Используется для создания ресурсов
- `PATCH`: Используется для обновления характеристик ресурсов
- `PUT`: Используется для замены ресурсов или коллекций ресурсов
- `DELETE`: Используется для удаления ресурсов
### Headers
Заголовки представляют доп сведения о запросе и требуемом ответе

#### `Accept`
Большинство конечных точек GitHub REST API указывают, что нужно передавать Accept заголовок со значением application/vnd.github+json. Значение заголовка Accept — тип носителя.
#### `X-GitHub-Api-Version`
Этот заголовок следует использовать для указания версии REST API, используемой для запроса.
#### `User-Agent`
Все запросы API должны содержать допустимый User-Agent заголовок. Заголовок User-Agent определяет пользователя или приложение, выполняющее запрос.
>[!warning]
Запросы без заголовка User-Agent отклоняются. Если указать недопустимый User-Agent заголовок, вы получите 403 Forbidden ответ.

### Аутентификация 
Для многих конечных точек требуется проверка подлинности или возврат дополнительных сведений при проверке подлинности. Кроме того, при проверке подлинности можно выполнять больше запросов в час.

Есть несколько способов получить токен:
- Создать personal access token
- Сгенерировать токен с GitHub App
- Использовать встроенный GITHUB_TOKEN токе в рабочем GitHub Actions процессе.
После создания можно выполнить проверку подлинности, отправив маркер в заголовке `Authorization` запроса. 
>[!example]
>```http
>curl --request GET \
--url "https://api.github.com/octocat" \
--header "Authorization: Bearer YOUR-TOKEN" \
--header "X-GitHub-Api-Version: 2026-03-10"
>```

>[!note]
>В большинстве случаев токен передается с помощью `Authorization: Bearer` или `Authorization: token`. Однако при передаче веб-токена JSON (JWT) необходимо использовать `Authorization: Bearer`.


### Параметры
Многие методы API требуют или позволяют отправлять дополнительные сведения в параметрах запроса. Существует несколько различных типов параметров: параметры пути, параметры тела и параметры запроса.
#### Параметры запроса
Параметры запроса позволяют передавать дополнительные данные в API. Эти параметры могут быть необязательными или обязательными в зависимости от конечной точки. Например, параметр body может позволить указать заголовок проблемы при создании новой проблемы или указать определенные параметры при включении или отключении функции.
## GitLab. Правила обращения
### Путь
Для GitLab API также нужен конкретный путь до endpoint.
- Запрос начинается с корневого endpoint (то есть Gitlab host name)
- Далее путь обязательно должен содержать в себе `/api/v4` (v4 - это версия API)
### HTTP Methods
Точно так же используются, как и в GitHub

### Request payload
Любой payload может быть обработан как query string, либо как payload body.
- Query string:
```http
curl --request POST \
  --url "https://gitlab.example.com/api/v4/projects?name=<example-name>&description=<example-description>"
```
- Request payload (JSON):
```http
curl --request POST \
  --header "Content-Type: application/json" \
  --data '{"name":"<example-name>", "description":"<example-description>"}' "https://gitlab.example.com/api/v4/projects"
```

>[!important]
>URL-закодированные query strings имеют лимит по длине. Запросы, которые превышают этот лимит, получают в ответ `414 Request-URI Too Large` сообщение об ошибке.


### Аутентификация
Многие API запросы требуют аутентификацию, или возвращают только публичные данные, если аутентификация не указана. 
В GitLab авторизоваться можно несколькими способами:
- OAuth 2.0 tokens
- Personal access tokens
- Project access tokens
- Group access tokens
- Session cookie
- CI/CD job tokens
>[!important]
>Токены можно вставлять как в url, так и в headers

#### OAuth 2.0 tokens
Его можно использовать как header
```http
--header "Authorization: Bearer OAUTH-TOKEN"
```
Либо прям в URL
```http
--url "https://gitlab.example.com/api/v4/projects?access_token=OAUTH-TOKEN"
```

#### Personal, project, and group access tokens
Их уже можно использовать только в headers, но несколькими способами
```http
--header "PRIVATE-TOKEN: <your_access_token>"
```
Либо
```http
--header "Authorization: Bearer <your_access_token>"
```
### Параметры
#### Path параметры
Если endpoint содержит в себе path параметры, то они в пути обозначаются как `:id` или `:group_id` .Данные параметры должны быть заменены на соответствующие им реальные значения.
>[!important]
>У некоторых параметров пути бывает как id, так и iid
>- id: ID которое является уникальным среди всех проектов
>- iid: Добавочный, внутренний ID (отображаемый в Web UI) который является уникальным только в рамках одного проекта
>
>Если ресурс поддерживает и id, и iid, то чаще всего используется iid вместо id, для получения ресурсов.

# 3. Получение данных о репозитории
## GitHub
>[!important]
В GitHub ключевым объектом является repository

### Пример запроса
```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO
```
`parent` и `source` объекты представляются, если репозиторий является форком. Тогда `parent` является репозиторием, с которого сделан форк, а `source` это ультимативный источник в сети.

### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.

### Параметры для запроса
#### Headers
- `accept` string
	Поставить `application/vnd.github+json` рекомендуется.

##### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.

#### HTTP response status codes

| Status code | Description        |
| ----------- | ------------------ |
| 200         | OK                 |
| 301         | Moved permanently  |
| 403         | Forbidden          |
| 404         | Resource not found |
>[!important]
>
Далее использую URL указаные в body ответа можно достать до всех оставшихся **endpoint**. 

## GitLab
>[!important]
>В GitLab ключевым объектом является PRoject

### Пример запроса
```http
GET /projects/:id
```

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
     --header "Accept: application/json" \
     --url "https://gitlab.example.com/api/v4/projects/<project_id>"
```

### Поддерживаемые атрибуты

| Атрибут                  | Тип           | Обязателен | Объяснение                                              |
| ------------------------ | ------------- | ---------- | ------------------------------------------------------- |
| `id`                     | int or string | Yes        | ID либо URL-encoded путь до объекта                     |
| `license`                | boolean       | No         | Вставлять информацию о лицензии                         |
| `statistics`             | boolean       | No         | Вставить статистику проекта. Не доступно обычному юзеру |
| `with_custom_attributes` | boolean       | No         | Только для админов. Не интересно                        |

### Если успешно, то ответ `200 OK` и ответ содержит следующие параметры

| Attribute                | Type              | Description |
|:-------------------------|:------------------|:------------|
| `id` | integer | ID of the project. |
| `description` | string | Description of the project. |
| `description_html` | string | Description of the project in HTML format. |
| `name` | string | Name of the project. |
| `name_with_namespace` | string | Name of the project with its namespace. |
| `path` | string | Path of the project. |
| `path_with_namespace` | string | Path of the project with its namespace. |
| `created_at` | datetime | Timestamp when the project was created. |
| `default_branch` | string | Default branch of the project. |
| `tag_list` | array of strings | Deprecated. Use `topics` instead. List of tags for the project. |
| `topics` | array of strings | List of topics for the project. |
| `ssh_url_to_repo` | string | SSH URL to clone the repository. |
| `http_url_to_repo` | string | HTTP URL to clone the repository. |
| `web_url` | string | URL to access the project in a browser. |
| `readme_url` | string | URL to the project's README file. |
| `forks_count` | integer | Number of forks of the project. |
| `avatar_url` | string | URL to the project's avatar image. |
| `star_count` | integer | Number of stars the project has received. |
| `last_activity_at` | datetime | Timestamp of the last activity in the project. |
| `visibility` | string | Visibility level of the project. Possible values: `private`, `internal`, or `public`. |
| `namespace` | object | Namespace information for the project. |
| `namespace.id` | integer | ID of the namespace. |
| `namespace.name` | string | Name of the namespace. |
| `namespace.path` | string | Path of the namespace. |
| `namespace.kind` | string | Type of namespace. Possible values: `user` or `group`. |
| `namespace.full_path` | string | Full path of the namespace. |
| `namespace.parent_id` | integer | ID of the parent namespace, if applicable. |
| `namespace.avatar_url` | string | URL to the namespace's avatar image. |
| `namespace.web_url` | string | URL to access the namespace in a browser. |
| `container_registry_image_prefix` | string | Prefix for container registry images. |
| `_links` | object | Collection of API endpoint links related to the project. |
| `_links.self` | string | URL to the project resource. |
| `_links.issues` | string | URL to the project's issues. |
| `_links.merge_requests` | string | URL to the project's merge requests. |
| `_links.repo_branches` | string | URL to the project's repository branches. |
| `_links.labels` | string | URL to the project's labels. |
| `_links.events` | string | URL to the project's events. |
| `_links.members` | string | URL to the project's members. |
| `_links.cluster_agents` | string | URL to the project's cluster agents. |
| `marked_for_deletion_at` | date | Deprecated. Use `marked_for_deletion_on` instead. Date when the project is scheduled for deletion. |
| `marked_for_deletion_on` | date | Date when the project is scheduled for deletion. |
| `packages_enabled` | boolean | Whether the package registry is enabled for the project. |
| `empty_repo` | boolean | Whether the repository is empty. |
| `archived` | boolean | Whether the project is archived. |
| `owner` | object | Information about the project owner. |
| `owner.id` | integer | ID of the project Owner. |
| `owner.username` | string | Username of the owner. |
| `owner.public_email` | string | Public email address of the owner. |
| `owner.name` | string | Name of the project Owner. |
| `owner.state` | string | Current state of the owner account. |
| `owner.locked` | boolean | Indicates if the owner account is locked. |
| `owner.avatar_url` | string | URL to the owner's avatar image. |
| `owner.web_url` | string | Web URL for the owner's profile. |
| `owner.created_at` | datetime | Timestamp when the Owner was created. |
| `resolve_outdated_diff_discussions` | boolean | Whether outdated diff discussions are automatically resolved. |
| `container_expiration_policy` | object | Settings for container image expiration policy. |
| `container_expiration_policy.cadence` | string | How often the container expiration policy runs. |
| `container_expiration_policy.enabled` | boolean | Whether the container expiration policy is enabled. |
| `container_expiration_policy.keep_n` | integer | Number of container images to keep. |
| `container_expiration_policy.older_than` | string | Remove container images older than this value. |
| `container_expiration_policy.name_regex` | string | Deprecated. Use `name_regex_delete` instead. Regular expression to match container image names. |
| `container_expiration_policy.name_regex_delete` | string | Regular expression to match container image names to delete. |
| `container_expiration_policy.name_regex_keep` | string | Regular expression to match container image names to keep. |
| `container_expiration_policy.next_run_at` | datetime | Timestamp for the next scheduled policy run. |
| `repository_object_format` | string | Object format used by the repository. Possible values: `sha1` or `sha256`. |
| `issues_enabled` | boolean | Whether issues are enabled for the project. |
| `merge_requests_enabled` | boolean | Whether merge requests are enabled for the project. |
| `wiki_enabled` | boolean | Whether the wiki is enabled for the project. |
| `jobs_enabled` | boolean | Whether jobs are enabled for the project. |
| `snippets_enabled` | boolean | Whether snippets are enabled for the project. |
| `container_registry_enabled` | boolean | Deprecated. Use `container_registry_access_level` instead. Whether the container registry is enabled. |
| `service_desk_enabled` | boolean | Whether Service Desk is enabled for the project. |
| `service_desk_address` | string | Email address for the Service Desk. |
| `can_create_merge_request_in` | boolean | Whether the current user can create merge requests in the project. |
| `issues_access_level` | string | Access level for the issues feature. Possible values: `disabled`, `private`, or `enabled`. |
| `repository_access_level` | string | Access level for the repository feature. Possible values: `disabled`, `private`, or `enabled`. |
| `merge_requests_access_level` | string | Access level for the merge requests feature. Possible values: `disabled`, `private`, or `enabled`. |
| `forking_access_level` | string | Access level for forking the project. Possible values: `disabled`, `private`, or `enabled`. |
| `wiki_access_level` | string | Access level for the wiki feature. Possible values: `disabled`, `private`, or `enabled`. |
| `builds_access_level` | string | Access level for the CI/CD builds feature. Possible values: `disabled`, `private`, or `enabled`. |
| `snippets_access_level` | string | Access level for the snippets feature. Possible values: `disabled`, `private`, or `enabled`. |
| `pages_access_level` | string | Access level for GitLab Pages. Possible values: `disabled`, `private`, `enabled`, or `public`. |
| `analytics_access_level` | string | Access level for analytics features. Possible values: `disabled`, `private`, or `enabled`. |
| `container_registry_access_level` | string | Access level for the container registry. Possible values: `disabled`, `private`, or `enabled`. |
| `security_and_compliance_access_level` | string | Access level for security and compliance features. Possible values: `disabled`, `private`, or `enabled`. |
| `releases_access_level` | string | Access level for the releases feature. Possible values: `disabled`, `private`, or `enabled`. |
| `environments_access_level` | string | Access level for the environments feature. Possible values: `disabled`, `private`, or `enabled`. |
| `feature_flags_access_level` | string | Access level for the feature flags feature. Possible values: `disabled`, `private`, or `enabled`. |
| `infrastructure_access_level` | string | Access level for the infrastructure feature. Possible values: `disabled`, `private`, or `enabled`. |
| `monitor_access_level` | string | Access level for the monitor feature. Possible values: `disabled`, `private`, or `enabled`. |
| `model_experiments_access_level` | string | Access level for the model experiments feature. Possible values: `disabled`, `private`, or `enabled`. |
| `model_registry_access_level` | string | Access level for the model registry feature. Possible values: `disabled`, `private`, or `enabled`. |
| `package_registry_access_level` | string | Access level for the package registry feature. Possible values: `disabled`, `private`, or `enabled`. |
| `emails_disabled` | boolean | Indicates if emails are disabled for the project. |
| `emails_enabled` | boolean | Indicates if emails are enabled for the project. |
| `show_diff_preview_in_email` | boolean | Indicates if diff previews are shown in email notifications. |
| `shared_runners_enabled` | boolean | Whether shared runners are enabled for the project. |
| `lfs_enabled` | boolean | Indicates if Git LFS is enabled for the project. |
| `creator_id` | integer | ID of the user who created the project. |
| `import_url` | string | URL the project was imported from. |
| `import_type` | string | Type of import used for the project. |
| `import_status` | string | Status of the project import. |
| `import_error` | string | Error message if the import failed. |
| `open_issues_count` | integer | Number of open issues. |
| `updated_at` | datetime | Timestamp when the project was last updated. |
| `ci_default_git_depth` | integer | Default Git depth for CI/CD pipelines. Only visible if you have administrator access or the Owner role for the project. |
| `ci_delete_pipelines_in_seconds` | integer | Time in seconds before old pipelines are deleted. |
| `ci_forward_deployment_enabled` | boolean | Whether forward deployment is enabled. Only visible if you have administrator access or the Owner role for the project. |
| `ci_forward_deployment_rollback_allowed` | boolean | Whether rollback is allowed for forward deployments. |
| `ci_job_token_scope_enabled` | boolean | Indicates if CI/CD job token scope is enabled. Only visible if you have administrator access or the Owner role for the project. |
| `ci_separated_caches` | boolean | Whether CI/CD caches are separated by branch. Only visible if you have administrator access or the Owner role for the project. |
| `ci_allow_fork_pipelines_to_run_in_parent_project` | boolean | Whether fork pipelines can run in the parent project. Only visible if you have administrator access or the Owner role for the project. |
| `ci_id_token_sub_claim_components` | array of strings | Components included in the CI/CD ID token subject claim. |
| `ci_skip_branch_pipelines_for_mrs` | boolean | Whether [branch pipelines are skipped for merge requests](../ci/pipelines/settings.md#skip-branch-pipelines-for-merge-requests) when the branch has an open merge request. Only visible if you have administrator access or the Owner role for the project. |
| `build_git_strategy` | string | Git strategy used for CI/CD builds (fetch or clone). Only visible if you have administrator access or the Owner role for the project. |
| `keep_latest_artifact` | boolean | Indicates if the latest artifact is kept when a new one is created. Only visible if you have administrator access or the Owner role for the project. |
| `restrict_user_defined_variables` | boolean | Whether user-defined variables are restricted. Only visible if you have administrator access or the Owner role for the project. |
| `ci_pipeline_variables_minimum_override_role` | string | Minimum role required to override pipeline variables. |
| `runner_token_expiration_interval` | integer | Expiration interval in seconds for runner tokens. Only visible if you have administrator access or the Owner role for the project. |
| `group_runners_enabled` | boolean | Whether group runners are enabled for the project. Only visible if you have administrator access or the Owner role for the project. |
| `resource_group_default_process_mode` | string | Default process mode for resource groups. |
| `auto_cancel_pending_pipelines` | string | Setting for automatically canceling pending pipelines. Only visible if you have administrator access or the Owner role for the project. |
| `build_timeout` | integer | Timeout in seconds for CI/CD jobs. Only visible if you have administrator access or the Owner role for the project. |
| `auto_devops_enabled` | boolean | Whether Auto DevOps is enabled for the project. Only visible if you have administrator access or the Owner role for the project. |
| `auto_devops_deploy_strategy` | string | Deployment strategy for Auto DevOps. Only visible if you have administrator access or the Owner role for the project. |
| `ci_push_repository_for_job_token_allowed` | boolean | Whether pushing to the repository is allowed using a job token. |
| `cicd_catalog_enabled` | boolean | Whether the project is published to the [CI/CD Catalog](../ci/components/_index.md#cicd-catalog). [Introduced](https://gitlab.com/gitlab-org/gitlab/-/issues/463043) in GitLab 19.3. |
| `runners_token` | string | Token for registering runners with the project. Only visible if you have administrator access or the Owner role for the project. |
| `ci_config_path` | string | Path to the CI/CD configuration file. |
| `public_jobs` | boolean | Whether job logs are publicly accessible. |
| `shared_with_groups` | array of objects | List of groups the project is shared with. |
| `shared_with_groups[].group_id` | integer | ID of the group the project is shared with. |
| `shared_with_groups[].group_name` | string | Name of the group the project is shared with. |
| `shared_with_groups[].group_full_path` | string | Full path of the group the project is shared with. |
| `shared_with_groups[].group_access_level` | integer | Access level granted to the group. |
| `only_allow_merge_if_pipeline_succeeds` | boolean | Whether merges are allowed only if the pipeline succeeds. |
| `allow_merge_on_skipped_pipeline` | boolean | Whether merges are allowed when the pipeline is skipped. |
| `request_access_enabled` | boolean | Whether users can request access to the project. |
| `only_allow_merge_if_all_discussions_are_resolved` | boolean | Whether merges are allowed only if all discussions are resolved. |
| `remove_source_branch_after_merge` | boolean | Whether the source branch is automatically removed after merge. |
| `printing_merge_request_link_enabled` | boolean | Indicates if merge request links are printed after pushing. |
| `printing_merge_requests_link_enabled` | boolean | Whether the merge request link is printed after a push. |
| `merge_method` | string | Merge method used for the project. Possible values: `merge`, `rebase_merge`, or `ff`. |
| `merge_request_title_regex` | string | Regex pattern for validating merge request titles. |
| `merge_request_title_regex_description` | string | Description of the merge request title regex validation. |
| `squash_option` | string | Squash option for merge requests. |
| `automatic_rebase_enabled` | boolean | Indicates if the source branch is automatically rebased before merge. |
| `enforce_auth_checks_on_uploads` | boolean | Whether authentication checks are enforced on uploads. |
| `suggestion_commit_message` | string | Custom commit message for suggestions. |
| `merge_commit_template` | string | Template for merge commit messages. |
| `mr_default_title_template` | string | Template for merge request titles. |
| `squash_commit_template` | string | Template for squash commit messages. |
| `issue_branch_template` | string | Template for branch names created from issues. |
| `warn_about_potentially_unwanted_characters` | boolean | Whether to warn about potentially unwanted characters. |
| `autoclose_referenced_issues` | boolean | Whether referenced issues are automatically closed. |
| `max_artifacts_size` | integer | Maximum size in MB for CI/CD artifacts. |
| `approvals_before_merge` | integer | Deprecated. Use merge request approvals API instead. Number of approvals required before merge. |
| `mirror` | boolean | Whether the project is a mirror. |
| `external_authorization_classification_label` | string | External authorization classification label. |
| `requirements_enabled` | boolean | Indicates if requirements management is enabled. |
| `requirements_access_level` | string | Access level for the requirements feature. |
| `security_and_compliance_enabled` | boolean | Indicates if security and compliance features are enabled. |
| `secret_push_protection_enabled` | boolean | Whether secret push protection is enabled. |
| `pre_receive_secret_detection_enabled` | boolean | Indicates if pre-receive secret detection is enabled. |
| `compliance_frameworks` | array of strings | Compliance frameworks applied to the project. |
| `issues_template` | string | Default description for issues. Description is parsed with GitLab Flavored Markdown. Premium and Ultimate only. |
| `merge_requests_template` | string | Template for merge request descriptions. Premium and Ultimate only. |
| `ci_restrict_pipeline_cancellation_role` | string | Minimum role required to cancel pipelines. |
| `merge_pipelines_enabled` | boolean | Indicates if merge pipelines are enabled. |
| `merge_trains_enabled` | boolean | Indicates if merge trains are enabled. |
| `merge_trains_skip_train_allowed` | boolean | Indicates if skipping the merge train is allowed. |
| `merge_train_enforcement` | string | Merge train enforcement level. One of `allow_bypass`, `enforce_for_all_users`, or `enforce_with_owner_override`. Has no effect unless merge trains are enabled for the project. |
| `max_pipelines_per_merge_train` | integer | Maximum number of parallel pipelines per merge train. |
| `only_allow_merge_if_all_status_checks_passed` | boolean | Whether merges are allowed only if all status checks have passed. Ultimate only. |
| `allow_pipeline_trigger_approve_deployment` | boolean | Whether pipeline triggers can approve deployments. |
| `prevent_merge_without_jira_issue` | boolean | Indicates if merges require an associated Jira issue. |
| `reviewer_assignment_strategy` | string | Strategy used to automatically assign reviewers to merge requests. One of `disabled` or `code_owners`. This attribute can also return `dap_powered` for projects configured before GitLab 19.4. Premium and Ultimate only. |
| `duo_remote_flows_enabled` | boolean | Indicates if GitLab Duo remote flows are enabled. |
| `duo_foundational_flows_enabled` | boolean | Indicates if GitLab Duo foundational flows are enabled. |
| `duo_sast_fp_detection_enabled` | boolean | Indicates if GitLab Duo SAST false positive detection is enabled. |
| `duo_sast_vr_workflow_enabled` | boolean | Indicates if GitLab Duo SAST vulnerability resolution workflow is enabled. |
| `web_based_commit_signing_enabled` | boolean | Indicates if web-based commit signing is enabled. |
| `spp_repository_pipeline_access` | boolean | Repository pipeline access for security policies. Only visible if the security orchestration policies feature is available. |
| `permissions` | object | User permissions for the project. |
| `permissions.project_access` | object | Project-level access permissions for the user. |
| `permissions.project_access.access_level` | integer | Access level for the project. |
| `permissions.project_access.notification_level` | integer | Notification level for the project. |
| `permissions.group_access` | object | Group-level access permissions for the user. |
| `permissions.group_access.access_level` | integer | Access level for the group. |
| `permissions.group_access.notification_level` | integer | Notification level for the group. |
| `license_url` | string | URL to the project's license file. |
| `license.key` | string | Key identifier for the license. |
| `license.name` | string | Full name of the license. |
| `license.nickname` | string | Nickname of the license. |
| `license.html_url` | string | URL to view the license details. |
| `license.source_url` | string | URL to the license source text. |
| `repository_storage` | string | Storage location for the project's repository. |
| `mirror_user_id` | integer | ID of the user who set up the mirror. |
| `mirror_trigger_builds` | boolean | Whether mirror updates trigger builds. |
| `only_mirror_protected_branches` | boolean | Whether only protected branches are mirrored. |
| `mirror_overwrites_diverged_branches` | boolean | Whether the mirror overwrites diverged branches. |
| `statistics.commit_count` | integer | Number of commits in the project. |
| `statistics.storage_size` | integer | Total storage size in bytes. |
| `statistics.repository_size` | integer | Repository storage size in bytes. |
| `statistics.wiki_size` | integer | Wiki storage size in bytes. |
| `statistics.lfs_objects_size` | integer | LFS objects storage size in bytes. |
| `statistics.job_artifacts_size` | integer | Job artifacts storage size in bytes. |
| `statistics.pipeline_artifacts_size` | integer | Pipeline artifacts storage size in bytes. |
| `statistics.packages_size` | integer | Packages storage size in bytes. |
| `statistics.snippets_size` | integer | Snippets storage size in bytes. |
| `statistics.uploads_size` | integer | Uploads storage size in bytes. |
| `statistics.container_registry_size` | integer | Total container registry storage size in bytes used by all container repositories in the project. Updates when container images are pushed or deleted. For GitLab Self-Managed instances, requires the container registry metadata database to be enabled. |
| `forked_from_project` | object | The upstream project this project was forked from. If the upstream project is private, an authentication token is required to view this field. |
| `mr_default_target_self` | boolean | Whether merge requests target this project by default. If `false`, merge requests target the upstream project. Appears only if the project is a fork. |

### Пример ответа
```json
{
  "id": 3,
  "description": "Lorem ipsum dolor sit amet, consectetur adipiscing elit.",
  "description_html": "<p data-sourcepos=\"1:1-1:56\" dir=\"auto\">Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>",
  "default_branch": "main",
  "visibility": "private",
  "ssh_url_to_repo": "git@example.com:diaspora/diaspora-project-site.git",
  "http_url_to_repo": "http://example.com/diaspora/diaspora-project-site.git",
  "web_url": "http://example.com/diaspora/diaspora-project-site",
  "readme_url": "http://example.com/diaspora/diaspora-project-site/blob/main/README.md",
  "tag_list": [ //deprecated, use `topics` instead
    "example",
    "disapora project"
  ],
  "topics": [
    "example",
    "disapora project"
  ],
  "owner": {
    "id": 3,
    "name": "Diaspora",
    "created_at": "2013-09-30T13:46:02Z"
  },
  "name": "Diaspora Project Site",
  "name_with_namespace": "Diaspora / Diaspora Project Site",
  "path": "diaspora-project-site",
  "path_with_namespace": "diaspora/diaspora-project-site",
  "issues_enabled": true,
  "open_issues_count": 1,
  "merge_requests_enabled": true,
  "jobs_enabled": true,
  "wiki_enabled": true,
  "snippets_enabled": false,
  "can_create_merge_request_in": true,
  "resolve_outdated_diff_discussions": false,
  "container_registry_enabled": false, // deprecated, use container_registry_access_level instead
  "container_registry_access_level": "disabled",
  "security_and_compliance_access_level": "disabled",
  "container_expiration_policy": {
    "cadence": "7d",
    "enabled": false,
    "keep_n": null,
    "older_than": null,
    "name_regex": null, // to be deprecated in GitLab 13.0 in favor of `name_regex_delete`
    "name_regex_delete": null,
    "name_regex_keep": null,
    "next_run_at": "2020-01-07T21:42:58.658Z"
  },
  "created_at": "2013-09-30T13:46:02Z",
  "updated_at": "2013-09-30T13:46:02Z",
  "last_activity_at": "2013-09-30T13:46:02Z",
  "creator_id": 3,
  "namespace": {
    "id": 3,
    "name": "Diaspora",
    "path": "diaspora",
    "kind": "group",
    "full_path": "diaspora",
    "avatar_url": "http://localhost:3000/uploads/group/avatar/3/foo.jpg",
    "web_url": "http://localhost:3000/groups/diaspora"
  },
  "import_url": null,
  "import_type": null,
  "import_status": "none",
  "import_error": null,
  "permissions": {
    "project_access": {
      "access_level": 10,
      "notification_level": 3
    },
    "group_access": {
      "access_level": 50,
      "notification_level": 3
    }
  },
  "archived": false,
  "avatar_url": "http://example.com/uploads/project/avatar/3/uploads/avatar.png",
  "license_url": "http://example.com/diaspora/diaspora-client/blob/main/LICENSE",
  "license": {
    "key": "lgpl-3.0",
    "name": "GNU Lesser General Public License v3.0",
    "nickname": "GNU LGPLv3",
    "html_url": "http://choosealicense.com/licenses/lgpl-3.0/",
    "source_url": "http://www.gnu.org/licenses/lgpl-3.0.txt"
  },
  "shared_runners_enabled": true,
  "group_runners_enabled": true,
  "forks_count": 0,
  "star_count": 0,
  "runners_token": "b8bc4a7a29eb76ea83cf79e4908c2b",
  "ci_default_git_depth": 50,
  "ci_forward_deployment_enabled": true,
  "ci_forward_deployment_rollback_allowed": true,
  "ci_allow_fork_pipelines_to_run_in_parent_project": true,
  "ci_id_token_sub_claim_components": ["project_path", "ref_type", "ref"],
  "ci_separated_caches": true,
  "ci_restrict_pipeline_cancellation_role": "developer",
  "ci_pipeline_variables_minimum_override_role": "maintainer",
  "ci_push_repository_for_job_token_allowed": false,
  "ci_display_pipeline_variables": false,
  "ci_skip_branch_pipelines_for_mrs": false,
  "cicd_catalog_enabled": false,
  "protect_merge_request_pipelines": true,
  "public_jobs": true,
  "shared_with_groups": [
    {
      "group_id": 4,
      "group_name": "Twitter",
      "group_full_path": "twitter",
      "group_access_level": 30
    },
    {
      "group_id": 3,
      "group_name": "Gitlab Org",
      "group_full_path": "gitlab-org",
      "group_access_level": 10
    }
  ],
  "repository_storage": "default",
  "only_allow_merge_if_pipeline_succeeds": false,
  "allow_merge_on_skipped_pipeline": false,
  "allow_pipeline_trigger_approve_deployment": false,
  "restrict_user_defined_variables": false,
  "only_allow_merge_if_all_discussions_are_resolved": false,
  "remove_source_branch_after_merge": false,
  "printing_merge_requests_link_enabled": true,
  "request_access_enabled": false,
  "merge_method": "merge",
  "squash_option": "default_on",
  "auto_devops_enabled": true,
  "auto_devops_deploy_strategy": "continuous",
  "approvals_before_merge": 0, // Deprecated. Use merge request approvals API instead.
  "mirror": false,
  "mirror_user_id": 45,
  "mirror_trigger_builds": false,
  "only_mirror_protected_branches": false,
  "mirror_overwrites_diverged_branches": false,
  "external_authorization_classification_label": null,
  "packages_enabled": true,
  "empty_repo": false,
  "service_desk_enabled": false,
  "service_desk_address": null,
  "autoclose_referenced_issues": true,
  "suggestion_commit_message": null,
  "enforce_auth_checks_on_uploads": true,
  "merge_commit_template": null,
  "mr_default_title_template": null,
  "squash_commit_template": null,
  "issue_branch_template": "gitlab/%{id}-%{title}",
  "marked_for_deletion_at": "2020-04-03", // Deprecated in favor of marked_for_deletion_on. Planned for removal in a future version of the REST API.
  "marked_for_deletion_on": "2020-04-03",
  "compliance_frameworks": [ "sox" ],
  "warn_about_potentially_unwanted_characters": true,
  "secret_push_protection_enabled": false,
  "statistics": {
    "commit_count": 37,
    "storage_size": 1038090,
    "repository_size": 1038090,
    "wiki_size" : 0,
    "lfs_objects_size": 0,
    "job_artifacts_size": 0,
    "pipeline_artifacts_size": 0,
    "packages_size": 0,
    "snippets_size": 0,
    "uploads_size": 0,
    "container_registry_size": 0
  },
  "container_registry_image_prefix": "registry.example.com/diaspora/diaspora-client",
  "_links": {
    "self": "http://example.com/api/v4/projects",
    "issues": "http://example.com/api/v4/projects/1/issues",
    "merge_requests": "http://example.com/api/v4/projects/1/merge_requests",
    "repo_branches": "http://example.com/api/v4/projects/1/repository_branches",
    "labels": "http://example.com/api/v4/projects/1/labels",
    "events": "http://example.com/api/v4/projects/1/events",
    "members": "http://example.com/api/v4/projects/1/members",
    "cluster_agents": "http://example.com/api/v4/projects/1/cluster_agents"
  },
  "spp_repository_pipeline_access": false // Only visible if the security_orchestration_policies feature is available
}
```



# 4. Получение данных о pull requests и merge requests
## GitHub
### List pull Requests
- List Pull Requests в конкретном репозитории.
	Возвращает все pull requests, доступные в публичных репозиториях с GitHub Free и GitHub Free для организаций, GitHub Pro и репзитории с legacy billing plans
#### Медиа форматы
Данный endpoint поддерживает несколько кастомных медиа форматов:
- application/vnd.github.raw+json: возвращает сырой markdown body. Ответ будет содержать body. Данный формат стоит по умолчанию, если не передать другой специфичный медиа формат.
- application/vnd.github.text+json: возвращает текст markdown body. Ответ будет содержать  body_text.
- application/vnd.github.html+json: возвращает HTML формат из  body's markdown. Ответ будет содержать body_html.
- application/vnd.github.full+json: Возвращает сырое, текстовое, и HTML представление. Ответ будет содержать body, body_text, and body_html.
#### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.

#### Параметры для запроса
##### Headers
- `accept` string
	Ставить `application/vnb.github+json` рекомендуется
##### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.
##### Query параметры
- `state` string
	Может быть как `open`, `closed` или `all` для фильтрации по `state`.
	По умолчанию: `open`
- `head` string
	Фильтрует pulls по head пользователю или head организации и названию ветки в формате `user:ref-name` или `organization:ref-name`. 
- `base` string
	Фильтрует по base ветке
- `sort` string.
	Как отсортировать результат запроса. `popularity` отсортирует по количеству комментов. `long-running` отсортирует по дате создания и ограничит вывод до pull requests, которые были открыты более чем месяц назад и имели активность в последний месяц.
	По умолчанию: `created`
	Может быть: `created`, `updated`, `popularity`, `long-running`
- `direction` string
	Направление сортировки. По дефолту: `desc` если сортировка по `created` или sort не был задан, иначе `asc`. 
- `per_page` integer
	Сколько результатов на страницу (max 100).
- `page` integer
	Номер страницы для  получения результата. 
	По умолчанию: 1.
#### HTTP response status codes

| Status code | Объяснение                                              |
| ----------- | ------------------------------------------------------- |
| `200`       | OK                                                      |
| `304`       | Not modified                                            |
| `422`       | Валидация failed или слишком много запросов к endpoint. |

#### Пример запроса
```http
GET /repos/{owner}/{repo}/pulls
```
```http
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/pulls
```
#### Пример ответа
 Status: 200
 ```json
 {
  "url": "https://api.github.com/repos/octocat/Hello-World/pulls/1347",
  "id": 1,
  "node_id": "MDExOlB1bGxSZXF1ZXN0MQ==",
  "html_url": "https://github.com/octocat/Hello-World/pull/1347",
  "diff_url": "https://github.com/octocat/Hello-World/pull/1347.diff",
  "patch_url": "https://github.com/octocat/Hello-World/pull/1347.patch",
  "issue_url": "https://api.github.com/repos/octocat/Hello-World/issues/1347",
  "commits_url": "https://api.github.com/repos/octocat/Hello-World/pulls/1347/commits",
  "review_comments_url": "https://api.github.com/repos/octocat/Hello-World/pulls/1347/comments",
  "review_comment_url": "https://api.github.com/repos/octocat/Hello-World/pulls/comments{/number}",
  "comments_url": "https://api.github.com/repos/octocat/Hello-World/issues/1347/comments",
  "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/6dcb09b5b57875f334f61aebed695e2e4193db5e",
  "number": 1347,
  "state": "open",
  "locked": true,
  "title": "Amazing new feature",
  "user": {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://api.github.com/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://api.github.com/users/octocat/followers",
    "following_url": "https://api.github.com/users/octocat/following{/other_user}",
    "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
    "organizations_url": "https://api.github.com/users/octocat/orgs",
    "repos_url": "https://api.github.com/users/octocat/repos",
    "events_url": "https://api.github.com/users/octocat/events{/privacy}",
    "received_events_url": "https://api.github.com/users/octocat/received_events",
    "type": "User",
    "site_admin": false
  },
  "body": "Please pull these awesome changes in!",
  "labels": [
    {
      "id": 208045946,
      "node_id": "MDU6TGFiZWwyMDgwNDU5NDY=",
      "url": "https://api.github.com/repos/octocat/Hello-World/labels/bug",
      "name": "bug",
      "description": "Something isn't working",
      "color": "f29513",
      "default": true
    }
  ],
  "milestone": {
    "url": "https://api.github.com/repos/octocat/Hello-World/milestones/1",
    "html_url": "https://github.com/octocat/Hello-World/milestones/v1.0",
    "labels_url": "https://api.github.com/repos/octocat/Hello-World/milestones/1/labels",
    "id": 1002604,
    "node_id": "MDk6TWlsZXN0b25lMTAwMjYwNA==",
    "number": 1,
    "state": "open",
    "title": "v1.0",
    "description": "Tracking milestone for version 1.0",
    "creator": {
      "login": "octocat",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/octocat_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/octocat",
      "html_url": "https://github.com/octocat",
      "followers_url": "https://api.github.com/users/octocat/followers",
      "following_url": "https://api.github.com/users/octocat/following{/other_user}",
      "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
      "organizations_url": "https://api.github.com/users/octocat/orgs",
      "repos_url": "https://api.github.com/users/octocat/repos",
      "events_url": "https://api.github.com/users/octocat/events{/privacy}",
      "received_events_url": "https://api.github.com/users/octocat/received_events",
      "type": "User",
      "site_admin": false
    },
    "open_issues": 4,
    "closed_issues": 8,
    "created_at": "2011-04-10T20:09:31Z",
    "updated_at": "2014-03-03T18:58:10Z",
    "closed_at": "2013-02-12T13:22:01Z",
    "due_on": "2012-10-09T23:39:01Z"
  },
  "active_lock_reason": "too heated",
  "created_at": "2011-01-26T19:01:12Z",
  "updated_at": "2011-01-26T19:01:12Z",
  "closed_at": "2011-01-26T19:01:12Z",
  "merged_at": "2011-01-26T19:01:12Z",
  "assignees": [
    {
      "login": "octocat",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/octocat_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/octocat",
      "html_url": "https://github.com/octocat",
      "followers_url": "https://api.github.com/users/octocat/followers",
      "following_url": "https://api.github.com/users/octocat/following{/other_user}",
      "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
      "organizations_url": "https://api.github.com/users/octocat/orgs",
      "repos_url": "https://api.github.com/users/octocat/repos",
      "events_url": "https://api.github.com/users/octocat/events{/privacy}",
      "received_events_url": "https://api.github.com/users/octocat/received_events",
      "type": "User",
      "site_admin": false
    },
    {
      "login": "hubot",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/hubot_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/hubot",
      "html_url": "https://github.com/hubot",
      "followers_url": "https://api.github.com/users/hubot/followers",
      "following_url": "https://api.github.com/users/hubot/following{/other_user}",
      "gists_url": "https://api.github.com/users/hubot/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/hubot/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/hubot/subscriptions",
      "organizations_url": "https://api.github.com/users/hubot/orgs",
      "repos_url": "https://api.github.com/users/hubot/repos",
      "events_url": "https://api.github.com/users/hubot/events{/privacy}",
      "received_events_url": "https://api.github.com/users/hubot/received_events",
      "type": "User",
      "site_admin": true
    }
  ],
  "requested_reviewers": [
    {
      "login": "other_user",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/other_user_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/other_user",
      "html_url": "https://github.com/other_user",
      "followers_url": "https://api.github.com/users/other_user/followers",
      "following_url": "https://api.github.com/users/other_user/following{/other_user}",
      "gists_url": "https://api.github.com/users/other_user/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/other_user/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/other_user/subscriptions",
      "organizations_url": "https://api.github.com/users/other_user/orgs",
      "repos_url": "https://api.github.com/users/other_user/repos",
      "events_url": "https://api.github.com/users/other_user/events{/privacy}",
      "received_events_url": "https://api.github.com/users/other_user/received_events",
      "type": "User",
      "site_admin": false
    }
  ],
  "requested_teams": [
    {
      "id": 1,
      "node_id": "MDQ6VGVhbTE=",
      "url": "https://api.github.com/teams/1",
      "html_url": "https://github.com/orgs/github/teams/justice-league",
      "name": "Justice League",
      "slug": "justice-league",
      "description": "A great team.",
      "privacy": "closed",
      "notification_setting": "notifications_enabled",
      "permission": "admin",
      "members_url": "https://api.github.com/teams/1/members{/member}",
      "repositories_url": "https://api.github.com/teams/1/repos"
    }
  ],
  "head": {
    "label": "octocat:new-topic",
    "ref": "new-topic",
    "sha": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
    "user": {
      "login": "octocat",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/octocat_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/octocat",
      "html_url": "https://github.com/octocat",
      "followers_url": "https://api.github.com/users/octocat/followers",
      "following_url": "https://api.github.com/users/octocat/following{/other_user}",
      "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
      "organizations_url": "https://api.github.com/users/octocat/orgs",
      "repos_url": "https://api.github.com/users/octocat/repos",
      "events_url": "https://api.github.com/users/octocat/events{/privacy}",
      "received_events_url": "https://api.github.com/users/octocat/received_events",
      "type": "User",
      "site_admin": false
    },
    "repo": {
      "id": 1296269,
      "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
      "name": "Hello-World",
      "full_name": "octocat/Hello-World",
      "owner": {
        "login": "octocat",
        "id": 1,
        "node_id": "MDQ6VXNlcjE=",
        "avatar_url": "https://github.com/images/error/octocat_happy.gif",
        "gravatar_id": "",
        "url": "https://api.github.com/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://api.github.com/users/octocat/followers",
        "following_url": "https://api.github.com/users/octocat/following{/other_user}",
        "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
        "organizations_url": "https://api.github.com/users/octocat/orgs",
        "repos_url": "https://api.github.com/users/octocat/repos",
        "events_url": "https://api.github.com/users/octocat/events{/privacy}",
        "received_events_url": "https://api.github.com/users/octocat/received_events",
        "type": "User",
        "site_admin": false
      },
      "private": false,
      "html_url": "https://github.com/octocat/Hello-World",
      "description": "This your first repo!",
      "fork": false,
      "url": "https://api.github.com/repos/octocat/Hello-World",
      "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
      "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
      "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
      "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
      "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
      "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
      "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
      "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
      "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
      "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
      "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
      "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
      "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
      "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
      "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
      "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
      "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
      "git_url": "git:github.com/octocat/Hello-World.git",
      "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
      "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
      "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
      "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
      "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
      "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
      "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
      "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
      "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
      "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
      "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
      "ssh_url": "git@github.com:octocat/Hello-World.git",
      "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
      "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
      "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
      "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
      "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
      "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
      "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
      "clone_url": "https://github.com/octocat/Hello-World.git",
      "mirror_url": "git:git.example.com/octocat/Hello-World",
      "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
      "svn_url": "https://svn.github.com/octocat/Hello-World",
      "homepage": "https://github.com",
      "language": null,
      "forks_count": 9,
      "stargazers_count": 80,
      "watchers_count": 80,
      "size": 108,
      "default_branch": "master",
      "open_issues_count": 0,
      "topics": [
        "octocat",
        "atom",
        "electron",
        "api"
      ],
      "has_issues": true,
      "has_projects": true,
      "has_wiki": true,
      "has_pages": false,
      "has_downloads": true,
      "has_discussions": false,
      "archived": false,
      "disabled": false,
      "pushed_at": "2011-01-26T19:06:43Z",
      "created_at": "2011-01-26T19:01:12Z",
      "updated_at": "2011-01-26T19:14:43Z",
      "permissions": {
        "admin": false,
        "push": false,
        "pull": true
      },
      "allow_rebase_merge": true,
      "temp_clone_token": "ABTLWHOULUVAXGTRYU7OC2876QJ2O",
      "allow_squash_merge": true,
      "allow_merge_commit": true,
      "allow_forking": true,
      "forks": 123,
      "open_issues": 123,
      "license": {
        "key": "mit",
        "name": "MIT License",
        "url": "https://api.github.com/licenses/mit",
        "spdx_id": "MIT",
        "node_id": "MDc6TGljZW5zZW1pdA=="
      },
      "watchers": 123
    }
  },
  "base": {
    "label": "octocat:master",
    "ref": "master",
    "sha": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
    "user": {
      "login": "octocat",
      "id": 1,
      "node_id": "MDQ6VXNlcjE=",
      "avatar_url": "https://github.com/images/error/octocat_happy.gif",
      "gravatar_id": "",
      "url": "https://api.github.com/users/octocat",
      "html_url": "https://github.com/octocat",
      "followers_url": "https://api.github.com/users/octocat/followers",
      "following_url": "https://api.github.com/users/octocat/following{/other_user}",
      "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
      "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
      "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
      "organizations_url": "https://api.github.com/users/octocat/orgs",
      "repos_url": "https://api.github.com/users/octocat/repos",
      "events_url": "https://api.github.com/users/octocat/events{/privacy}",
      "received_events_url": "https://api.github.com/users/octocat/received_events",
      "type": "User",
      "site_admin": false
    },
    "repo": {
      "id": 1296269,
      "node_id": "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
      "name": "Hello-World",
      "full_name": "octocat/Hello-World",
      "owner": {
        "login": "octocat",
        "id": 1,
        "node_id": "MDQ6VXNlcjE=",
        "avatar_url": "https://github.com/images/error/octocat_happy.gif",
        "gravatar_id": "",
        "url": "https://api.github.com/users/octocat",
        "html_url": "https://github.com/octocat",
        "followers_url": "https://api.github.com/users/octocat/followers",
        "following_url": "https://api.github.com/users/octocat/following{/other_user}",
        "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
        "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
        "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
        "organizations_url": "https://api.github.com/users/octocat/orgs",
        "repos_url": "https://api.github.com/users/octocat/repos",
        "events_url": "https://api.github.com/users/octocat/events{/privacy}",
        "received_events_url": "https://api.github.com/users/octocat/received_events",
        "type": "User",
        "site_admin": false
      },
      "private": false,
      "html_url": "https://github.com/octocat/Hello-World",
      "description": "This your first repo!",
      "fork": false,
      "url": "https://api.github.com/repos/octocat/Hello-World",
      "archive_url": "https://api.github.com/repos/octocat/Hello-World/{archive_format}{/ref}",
      "assignees_url": "https://api.github.com/repos/octocat/Hello-World/assignees{/user}",
      "blobs_url": "https://api.github.com/repos/octocat/Hello-World/git/blobs{/sha}",
      "branches_url": "https://api.github.com/repos/octocat/Hello-World/branches{/branch}",
      "collaborators_url": "https://api.github.com/repos/octocat/Hello-World/collaborators{/collaborator}",
      "comments_url": "https://api.github.com/repos/octocat/Hello-World/comments{/number}",
      "commits_url": "https://api.github.com/repos/octocat/Hello-World/commits{/sha}",
      "compare_url": "https://api.github.com/repos/octocat/Hello-World/compare/{base}...{head}",
      "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/{+path}",
      "contributors_url": "https://api.github.com/repos/octocat/Hello-World/contributors",
      "deployments_url": "https://api.github.com/repos/octocat/Hello-World/deployments",
      "downloads_url": "https://api.github.com/repos/octocat/Hello-World/downloads",
      "events_url": "https://api.github.com/repos/octocat/Hello-World/events",
      "forks_url": "https://api.github.com/repos/octocat/Hello-World/forks",
      "git_commits_url": "https://api.github.com/repos/octocat/Hello-World/git/commits{/sha}",
      "git_refs_url": "https://api.github.com/repos/octocat/Hello-World/git/refs{/sha}",
      "git_tags_url": "https://api.github.com/repos/octocat/Hello-World/git/tags{/sha}",
      "git_url": "git:github.com/octocat/Hello-World.git",
      "issue_comment_url": "https://api.github.com/repos/octocat/Hello-World/issues/comments{/number}",
      "issue_events_url": "https://api.github.com/repos/octocat/Hello-World/issues/events{/number}",
      "issues_url": "https://api.github.com/repos/octocat/Hello-World/issues{/number}",
      "keys_url": "https://api.github.com/repos/octocat/Hello-World/keys{/key_id}",
      "labels_url": "https://api.github.com/repos/octocat/Hello-World/labels{/name}",
      "languages_url": "https://api.github.com/repos/octocat/Hello-World/languages",
      "merges_url": "https://api.github.com/repos/octocat/Hello-World/merges",
      "milestones_url": "https://api.github.com/repos/octocat/Hello-World/milestones{/number}",
      "notifications_url": "https://api.github.com/repos/octocat/Hello-World/notifications{?since,all,participating}",
      "pulls_url": "https://api.github.com/repos/octocat/Hello-World/pulls{/number}",
      "releases_url": "https://api.github.com/repos/octocat/Hello-World/releases{/id}",
      "ssh_url": "git@github.com:octocat/Hello-World.git",
      "stargazers_url": "https://api.github.com/repos/octocat/Hello-World/stargazers",
      "statuses_url": "https://api.github.com/repos/octocat/Hello-World/statuses/{sha}",
      "subscribers_url": "https://api.github.com/repos/octocat/Hello-World/subscribers",
      "subscription_url": "https://api.github.com/repos/octocat/Hello-World/subscription",
      "tags_url": "https://api.github.com/repos/octocat/Hello-World/tags",
      "teams_url": "https://api.github.com/repos/octocat/Hello-World/teams",
      "trees_url": "https://api.github.com/repos/octocat/Hello-World/git/trees{/sha}",
      "clone_url": "https://github.com/octocat/Hello-World.git",
      "mirror_url": "git:git.example.com/octocat/Hello-World",
      "hooks_url": "https://api.github.com/repos/octocat/Hello-World/hooks",
      "svn_url": "https://svn.github.com/octocat/Hello-World",
      "homepage": "https://github.com",
      "language": null,
      "forks_count": 9,
      "stargazers_count": 80,
      "watchers_count": 80,
      "size": 108,
      "default_branch": "master",
      "open_issues_count": 0,
      "topics": [
        "octocat",
        "atom",
        "electron",
        "api"
      ],
      "has_issues": true,
      "has_projects": true,
      "has_wiki": true,
      "has_pages": false,
      "has_downloads": true,
      "has_discussions": false,
      "archived": false,
      "disabled": false,
      "pushed_at": "2011-01-26T19:06:43Z",
      "created_at": "2011-01-26T19:01:12Z",
      "updated_at": "2011-01-26T19:14:43Z",
      "permissions": {
        "admin": false,
        "push": false,
        "pull": true
      },
      "allow_rebase_merge": true,
      "temp_clone_token": "ABTLWHOULUVAXGTRYU7OC2876QJ2O",
      "allow_squash_merge": true,
      "allow_merge_commit": true,
      "forks": 123,
      "open_issues": 123,
      "license": {
        "key": "mit",
        "name": "MIT License",
        "url": "https://api.github.com/licenses/mit",
        "spdx_id": "MIT",
        "node_id": "MDc6TGljZW5zZW1pdA=="
      },
      "watchers": 123
    }
  },
  "_links": {
    "self": {
      "href": "https://api.github.com/repos/octocat/Hello-World/pulls/1347"
    },
    "html": {
      "href": "https://github.com/octocat/Hello-World/pull/1347"
    },
    "issue": {
      "href": "https://api.github.com/repos/octocat/Hello-World/issues/1347"
    },
    "comments": {
      "href": "https://api.github.com/repos/octocat/Hello-World/issues/1347/comments"
    },
    "review_comments": {
      "href": "https://api.github.com/repos/octocat/Hello-World/pulls/1347/comments"
    },
    "review_comment": {
      "href": "https://api.github.com/repos/octocat/Hello-World/pulls/comments{/number}"
    },
    "commits": {
      "href": "https://api.github.com/repos/octocat/Hello-World/pulls/1347/commits"
    },
    "statuses": {
      "href": "https://api.github.com/repos/octocat/Hello-World/statuses/6dcb09b5b57875f334f61aebed695e2e4193db5e"
    }
  },
  "author_association": "OWNER",
  "auto_merge": null,
  "draft": false,
  "merged": false,
  "mergeable": true,
  "rebaseable": true,
  "mergeable_state": "clean",
  "merged_by": {
    "login": "octocat",
    "id": 1,
    "node_id": "MDQ6VXNlcjE=",
    "avatar_url": "https://github.com/images/error/octocat_happy.gif",
    "gravatar_id": "",
    "url": "https://api.github.com/users/octocat",
    "html_url": "https://github.com/octocat",
    "followers_url": "https://api.github.com/users/octocat/followers",
    "following_url": "https://api.github.com/users/octocat/following{/other_user}",
    "gists_url": "https://api.github.com/users/octocat/gists{/gist_id}",
    "starred_url": "https://api.github.com/users/octocat/starred{/owner}{/repo}",
    "subscriptions_url": "https://api.github.com/users/octocat/subscriptions",
    "organizations_url": "https://api.github.com/users/octocat/orgs",
    "repos_url": "https://api.github.com/users/octocat/repos",
    "events_url": "https://api.github.com/users/octocat/events{/privacy}",
    "received_events_url": "https://api.github.com/users/octocat/received_events",
    "type": "User",
    "site_admin": false
  },
  "comments": 10,
  "review_comments": 0,
  "maintainer_can_modify": true,
  "commits": 3,
  "additions": 100,
  "deletions": 3,
  "changed_files": 5
}
 ```


## GitLab
### List Project Merge Requests
	Вывод все merge requests проекта.

#### Примеры запросов:
```http
GET /projects/:id/merge_requests
GET /projects/:id/merge_requests?state=opened
GET /projects/:id/merge_requests?state=all
GET /projects/:id/merge_requests?iids[]=42&iids[]=43
GET /projects/:id/merge_requests?milestone=release
GET /projects/:id/merge_requests?labels=bug,reproduced
GET /projects/:id/merge_requests?my_reaction_emoji=star
```

```http
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/1/merge_requests"
```
#### Поддерживаемые атрибуты

| Атрибут          | Тип                | Необходим | Объяснение                                                                                                                                                                 |
| ---------------- | ------------------ | --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`             | integer или string | Да        | ID или URL-encoded path проекта                                                                                                                                            |
| `iids[]`         | integer array      | Нет       | Возвращает mr, которые содержаться в этом array                                                                                                                            |
| `created_after`  | datetime           | Нет       | Возвращает mr, которые были созданы во время или после указанного времени. Ожидается в формате ISO 8601 (2019-03-15T08:00:00Z)                                             |
| `created_before` | datetime           | No        | Возвращает mr, которые были созданы во время или до указанного времени. Ожидается в формате ISO 8601 (2019-03-15T08:00:00Z)                                                |
| `order_by`       | string             | Нет       | Возвращает mr упорядоченные по (`created_at`, `updated_at`, `merged_at`, `label_priority`, `priority`, `milestone_due`, `popularity`, `title`). По умолчанию: `created_at` |
| `sort`           | string             | Нет       | Возвращает mr отсортированны в `asc` или `desc` последовательности. По умолчанию `desc`                                                                                    |
| `state`          | string             | Нет       | Возвращает все mr или некоторые, которые `opened`, `closed`, `locked`, `merged`. По умолчанию: `all`                                                                       |
| `update_after`   | datetime           | Нет       | Возвращает mr, которые были обновлены во время или после указанного времени. Ожидается в формате ISO 8601 (2019-03-15T08:00:00Z)                                           |
| `updated_before` | datetime           | Нет       | Возвращает mr, которые были обновлены во время или до указанного времени. Ожидается в формате ISO 8601 (2019-03-15T08:00:00Z)                                              |
| `view`           | string             | Нет       | Если `simple`, то возвращает `iid`, URL, title, description и базовое состояние mr                                                                                         |
|                  |                    |           |                                                                                                                                                                            |
#### Пример ответа
```json
[
  {
    "id": 1,
    "iid": 1,
    "project_id": 3,
    "title": "test1",
    "description": "fixed login page css paddings",
    "state": "merged",
    "imported": false,
    "imported_from": "none",
    "merged_by": { // Deprecated and will be removed in API v5, use `merge_user` instead
      "id": 87854,
      "name": "Douwe Maan",
      "username": "DouweM",
      "state": "active",
      "locked": false,
      "avatar_url": "https://gitlab.example.com/uploads/-/system/user/avatar/87854/avatar.png",
      "web_url": "https://gitlab.com/DouweM"
    },
    "merge_user": {
      "id": 87854,
      "name": "Douwe Maan",
      "username": "DouweM",
      "state": "active",
      "locked": false,
      "avatar_url": "https://gitlab.example.com/uploads/-/system/user/avatar/87854/avatar.png",
      "web_url": "https://gitlab.com/DouweM"
    },
    "merged_at": "2018-09-07T11:16:17.520Z",
    "merge_after": "2018-09-07T11:16:00.000Z",
    "prepared_at": "2018-09-04T11:16:17.520Z",
    "closed_by": null,
    "closed_at": null,
    "created_at": "2017-04-29T08:46:00Z",
    "updated_at": "2017-04-29T08:46:00Z",
    "target_branch": "main",
    "source_branch": "test1",
    "upvotes": 0,
    "downvotes": 0,
    "author": {
      "id": 1,
      "name": "Administrator",
      "username": "admin",
      "state": "active",
      "locked": false,
      "avatar_url": null,
      "web_url" : "https://gitlab.example.com/admin"
    },
    "assignee": {
      "id": 1,
      "name": "Administrator",
      "username": "admin",
      "state": "active",
      "locked": false,
      "avatar_url": null,
      "web_url" : "https://gitlab.example.com/admin"
    },
    "assignees": [{
      "name": "Miss Monserrate Beier",
      "username": "axel.block",
      "id": 12,
      "state": "active",
      "locked": false,
      "avatar_url": "http://www.gravatar.com/avatar/46f6f7dc858ada7be1853f7fb96e81da?s=80&d=identicon",
      "web_url": "https://gitlab.example.com/axel.block"
    }],
    "reviewers": [{
      "id": 2,
      "name": "Sam Bauch",
      "username": "kenyatta_oconnell",
      "state": "active",
      "avatar_url": "https://www.gravatar.com/avatar/956c92487c6f6f7616b536927e22c9a0?s=80&d=identicon",
      "web_url": "http://gitlab.example.com//kenyatta_oconnell"
    }],
    "source_project_id": 2,
    "target_project_id": 3,
    "labels": [
      "Community contribution",
      "Manage"
    ],
    "draft": false,
    "work_in_progress": false,
    "milestone": {
      "id": 5,
      "iid": 1,
      "project_id": 3,
      "title": "v2.0",
      "description": "Assumenda aut placeat expedita exercitationem labore sunt enim earum.",
      "state": "closed",
      "created_at": "2015-02-02T19:49:26.013Z",
      "updated_at": "2015-02-02T19:49:26.013Z",
      "due_date": "2018-09-22",
      "start_date": "2018-08-08",
      "web_url": "https://gitlab.example.com/my-group/my-project/milestones/1"
    },
    "merge_when_pipeline_succeeds": true,
    "merge_status": "can_be_merged",
    "detailed_merge_status": "not_open",
    "sha": "8888888888888888888888888888888888888888",
    "merge_commit_sha": null,
    "squash_commit_sha": null,
    "user_notes_count": 1,
    "discussion_locked": null,
    "should_remove_source_branch": true,
    "force_remove_source_branch": false,
    "web_url": "http://gitlab.example.com/my-group/my-project/merge_requests/1",
    "reference": "!1",
    "references": {
      "short": "!1",
      "relative": "!1",
      "full": "my-group/my-project!1"
    },
    "time_stats": {
      "time_estimate": 0,
      "total_time_spent": 0,
      "human_time_estimate": null,
      "human_total_time_spent": null
    },
    "squash": false,
    "squash_on_merge": false,
    "task_completion_status":{
      "count":0,
      "completed_count":0
    },
    "has_conflicts": false,
    "blocking_discussions_resolved": true,
    "approvals_before_merge": 2
  }
]
```
Если успешно, то 200 и со следующими атрибутами:

| Атрибут                     | Тип     | Объяснение                                                        |
| --------------------------- | ------- | ----------------------------------------------------------------- |
| `[].id`                     | integer | ID mr                                                             |
| `[].iid`                    | integer | другой ID mr                                                      |
| `[].approvals_before_merge` | integer | Number of approvals required before this merge request can merge. |
| `[].assignee`               | object  | First assignee of the merge request.                              |
| `[].assignees`              | array   | Assignees of the merge request.                                   |
|                             |         |                                                                   |
|                             |         |                                                                   |

# 5. Получение данных о имененных файлов в Pull Request и Merge Request
## GitHub
### List pull requests files
Выводит files конкретного pull request
#### Медиа форматы
Данный endpoint поддерживает несколько кастомных медиа форматов:
- application/vnd.github.raw+json: возвращает сырой markdown body. Ответ будет содержать body. Данный формат стоит по умолчанию, если не передать другой специфичный медиа формат.
- application/vnd.github.text+json: возвращает текст markdown body. Ответ будет содержать  body_text.
- application/vnd.github.html+json: возвращает HTML формат из  body's markdown. Ответ будет содержать body_html.
- application/vnd.github.full+json: Возвращает сырое, текстовое, и HTML представление. Ответ будет содержать body, body_text, and body_html.

#### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.

#### Параметры для запроса
##### Headers
- `accept` string
	Ставить `application/vnb.github+json` рекомендуется
##### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.
- `pull_number` integer ==Необходимый==
	Номер pull request
##### Query параметры
- `per_page` integer
	Сколько результатов на страницу (max 100).
- `page` integer
	Номер страницы для  получения результата. 
	По умолчанию: 1.

#### HTTP response status codes

| Status code | Description                                         |
| ----------- | --------------------------------------------------- |
| 200         | OK                                                  |
| 422         | Validation failed, or the endpoint has been spammed |
| 500         | Internal Error                                      |
| 503         | Service unavailable                                 |
#### Пример запроса
```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/pulls/PULL_NUMBER/files
```
#### Пример ответа
```json
[
  {
    "sha": "bbcd538c8e72b8c175046e27cc8f907076331401",
    "filename": "file1.txt",
    "status": "added",
    "additions": 103,
    "deletions": 21,
    "changes": 124,
    "blob_url": "https://github.com/octocat/Hello-World/blob/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
    "raw_url": "https://github.com/octocat/Hello-World/raw/6dcb09b5b57875f334f61aebed695e2e4193db5e/file1.txt",
    "contents_url": "https://api.github.com/repos/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
    "patch": "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
  }
]
```
## GitLab
>[!important]
>В GitLab можно вывести информацию по diffs

### List merge request diffs

List diffs of the files changed in a merge request.

```plaintext
GET /projects/:id/merge_requests/:merge_request_iid/diffs
```

#### Поддерживаемые атрибуты:

| Attribute           | Type              | Required | Description |
|---------------------|-------------------|----------|-------------|
| `id`                | integer or string | Yes      | The ID or [URL-encoded path of the project](rest/_index.md#namespaced-paths). |
| `merge_request_iid` | integer           | Yes      | The internal ID of the merge request. |
| `page`              | integer           | No       | The page of results to return. Defaults to 1. |
| `per_page`          | integer           | No       | The number of results per page. Defaults to 20. |
| `unidiff`           | boolean           | No       | Present diffs in the [unified diff](https://www.gnu.org/software/diffutils/manual/html_node/Detailed-Unified.html) format. Default is false. |

#### Если удачно, то возвращается [`200 OK`](rest/troubleshooting.md#status-codes) со следующими параметрами:

| Attribute        | Type    | Description |
|------------------|---------|-------------|
| `a_mode`         | string  | Old file mode of the file. |
| `b_mode`         | string  | New file mode of the file. |
| `collapsed`      | boolean | File diffs are excluded but can be fetched on request. |
| `deleted_file`   | boolean | File has been removed. |
| `diff`           | string  | Diff representation of the changes made to the file. |
| `generated_file` | boolean | File is [marked as generated](../user/project/merge_requests/changes.md#collapse-generated-files). |
| `new_file`       | boolean | File has been added. |
| `new_path`       | string  | New path of the file. |
| `old_path`       | string  | Old path of the file. |
| `renamed_file`   | boolean | File has been renamed. |
| `too_large`      | boolean | File diffs are excluded and cannot be retrieved. |

#### Пример запроса:

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/1/merge_requests/1/diffs?page=1&per_page=2"
```

#### Пример ответа:

```json
[
  {
    "old_path": "README",
    "new_path": "README",
    "a_mode": "100644",
    "b_mode": "100644",
    "diff": "@@ -1 +1 @@\ -Title\ +README",
    "collapsed": false,
    "too_large": false,
    "new_file": false,
    "renamed_file": false,
    "deleted_file": false,
    "generated_file": false
  },
  {
    "old_path": "VERSION",
    "new_path": "VERSION",
    "a_mode": "100644",
    "b_mode": "100644",
    "diff": "@@\ -1.9.7\ +1.9.8",
    "collapsed": false,
    "too_large": false,
    "new_file": false,
    "renamed_file": false,
    "deleted_file": false,
    "generated_file": false
  }
]
```
# 6. Получение commits к репозиторию GitHub и GitLab
## GitHub
### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.

### Параметры запроса
#### Headers
- `accept` string
	Ставить `application/vnb.github+json` рекомендуется
#### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.
#### Query параметры
- `sha` string
	SHA или branch с которого идет вывод коммитов. По дефолту это Default branch
- `path` string
	Только коммиты, которые содержат конкретный путь к файлу.
- `author` string
	GitHub username или email чтобы отфильтровать коммиты
- `committer` string
	GitHub username или email чтобы отфильтровать коммиты по коммитеру
- `since` string
	Возвращает только результаты, которые были обновлены после указаного времени. Формат timestamp - ISO 8601
- `until` string
	Возвращает только результаты, которые были до указаного времени. Формат timestamp - ISO 8601
- `per_page` integer
	Сколько результатов на странице (100 max)
- `page` integer
	 Номер страницы. По умолчанию: 1.

### HTTP response status code

| Status code | Description                                                                    |
| ----------- | ------------------------------------------------------------------------------ |
| 200         | OK                                                                             |
| 400         | Bad Request                                                                    |
| 404         | Resource not found                                                             |
| 409         | Conflict                                                                       |
| 422         | Validation failed, or the endpoint has been spammed                            |
| 429         | Запрос не может быть обработан из-за загруженности сервера. Попробуйте позднее |
| 500         | Internal Error                                                                 |
### Пример запроса
```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/commits
```

### Пример ответа
```json
{
  "type": "array",
  "items": {
    "title": "Commit",
    "description": "Commit",
    "type": "object",
    "properties": {
      "url": {
        "type": "string",
        "format": "uri"
      },
      "sha": {
        "type": "string"
      },
      "node_id": {
        "type": "string"
      },
      "html_url": {
        "type": "string",
        "format": "uri"
      },
      "comments_url": {
        "type": "string",
        "format": "uri"
      },
      "commit": {
        "type": "object",
        "properties": {
          "url": {
            "type": "string",
            "format": "uri"
          },
          "author": {
            "anyOf": [
              {
                "type": "null"
              },
              {
                "title": "Git User",
                "description": "Metaproperties for Git author/committer information.",
                "type": "object",
                "properties": {
                  "name": {
                    "type": "string"
                  },
                  "email": {
                    "type": "string"
                  },
                  "date": {
                    "type": "string",
                    "format": "date-time"
                  }
                }
              }
            ]
          },
          "committer": {
            "anyOf": [
              {
                "type": "null"
              },
              {
                "title": "Git User",
                "description": "Metaproperties for Git author/committer information.",
                "type": "object",
                "properties": {
                  "name": {
                    "type": "string"
                  },
                  "email": {
                    "type": "string"
                  },
                  "date": {
                    "type": "string",
                    "format": "date-time"
                  }
                }
              }
            ]
          },
          "message": {
            "type": "string"
          },
          "comment_count": {
            "type": "integer"
          },
          "tree": {
            "type": "object",
            "properties": {
              "sha": {
                "type": "string"
              },
              "url": {
                "type": "string",
                "format": "uri"
              }
            },
            "required": [
              "sha",
              "url"
            ]
          },
          "verification": {
            "title": "Verification",
            "type": "object",
            "properties": {
              "verified": {
                "type": "boolean"
              },
              "reason": {
                "type": "string"
              },
              "payload": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "signature": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "verified_at": {
                "type": [
                  "string",
                  "null"
                ]
              }
            },
            "required": [
              "verified",
              "reason",
              "payload",
              "signature",
              "verified_at"
            ]
          }
        },
        "required": [
          "author",
          "committer",
          "comment_count",
          "message",
          "tree",
          "url"
        ]
      },
      "author": {
        "oneOf": [
          {
            "title": "Simple User",
            "description": "A GitHub user.",
            "type": "object",
            "properties": {
              "name": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "email": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "login": {
                "type": "string"
              },
              "id": {
                "type": "integer",
                "format": "int64"
              },
              "node_id": {
                "type": "string"
              },
              "avatar_url": {
                "type": "string",
                "format": "uri"
              },
              "gravatar_id": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "url": {
                "type": "string",
                "format": "uri"
              },
              "html_url": {
                "type": "string",
                "format": "uri"
              },
              "followers_url": {
                "type": "string",
                "format": "uri"
              },
              "following_url": {
                "type": "string"
              },
              "gists_url": {
                "type": "string"
              },
              "starred_url": {
                "type": "string"
              },
              "subscriptions_url": {
                "type": "string",
                "format": "uri"
              },
              "organizations_url": {
                "type": "string",
                "format": "uri"
              },
              "repos_url": {
                "type": "string",
                "format": "uri"
              },
              "events_url": {
                "type": "string"
              },
              "received_events_url": {
                "type": "string",
                "format": "uri"
              },
              "type": {
                "type": "string"
              },
              "site_admin": {
                "type": "boolean"
              },
              "starred_at": {
                "type": "string"
              },
              "user_view_type": {
                "type": "string"
              }
            },
            "required": [
              "avatar_url",
              "events_url",
              "followers_url",
              "following_url",
              "gists_url",
              "gravatar_id",
              "html_url",
              "id",
              "node_id",
              "login",
              "organizations_url",
              "received_events_url",
              "repos_url",
              "site_admin",
              "starred_url",
              "subscriptions_url",
              "type",
              "url"
            ]
          },
          {
            "title": "Empty Object",
            "description": "An object without any properties.",
            "type": "object",
            "properties": {},
            "additionalProperties": false
          }
        ],
        "type": [
          "null",
          "object"
        ]
      },
      "committer": {
        "oneOf": [
          {
            "title": "Simple User",
            "description": "A GitHub user.",
            "type": "object",
            "properties": {
              "name": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "email": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "login": {
                "type": "string"
              },
              "id": {
                "type": "integer",
                "format": "int64"
              },
              "node_id": {
                "type": "string"
              },
              "avatar_url": {
                "type": "string",
                "format": "uri"
              },
              "gravatar_id": {
                "type": [
                  "string",
                  "null"
                ]
              },
              "url": {
                "type": "string",
                "format": "uri"
              },
              "html_url": {
                "type": "string",
                "format": "uri"
              },
              "followers_url": {
                "type": "string",
                "format": "uri"
              },
              "following_url": {
                "type": "string"
              },
              "gists_url": {
                "type": "string"
              },
              "starred_url": {
                "type": "string"
              },
              "subscriptions_url": {
                "type": "string",
                "format": "uri"
              },
              "organizations_url": {
                "type": "string",
                "format": "uri"
              },
              "repos_url": {
                "type": "string",
                "format": "uri"
              },
              "events_url": {
                "type": "string"
              },
              "received_events_url": {
                "type": "string",
                "format": "uri"
              },
              "type": {
                "type": "string"
              },
              "site_admin": {
                "type": "boolean"
              },
              "starred_at": {
                "type": "string"
              },
              "user_view_type": {
                "type": "string"
              }
            },
            "required": [
              "avatar_url",
              "events_url",
              "followers_url",
              "following_url",
              "gists_url",
              "gravatar_id",
              "html_url",
              "id",
              "node_id",
              "login",
              "organizations_url",
              "received_events_url",
              "repos_url",
              "site_admin",
              "starred_url",
              "subscriptions_url",
              "type",
              "url"
            ]
          },
          {
            "title": "Empty Object",
            "description": "An object without any properties.",
            "type": "object",
            "properties": {},
            "additionalProperties": false
          }
        ],
        "type": [
          "null",
          "object"
        ]
      },
      "parents": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "sha": {
              "type": "string"
            },
            "url": {
              "type": "string",
              "format": "uri"
            },
            "html_url": {
              "type": "string",
              "format": "uri"
            }
          },
          "required": [
            "sha",
            "url"
          ]
        }
      },
      "stats": {
        "type": "object",
        "properties": {
          "additions": {
            "type": "integer"
          },
          "deletions": {
            "type": "integer"
          },
          "total": {
            "type": "integer"
          }
        }
      },
      "files": {
        "type": "array",
        "items": {
          "title": "Diff Entry",
          "description": "Diff Entry",
          "type": "object",
          "properties": {
            "sha": {
              "type": [
                "string",
                "null"
              ]
            },
            "filename": {
              "type": "string"
            },
            "status": {
              "type": "string",
              "enum": [
                "added",
                "removed",
                "modified",
                "renamed",
                "copied",
                "changed",
                "unchanged"
              ]
            },
            "additions": {
              "type": "integer"
            },
            "deletions": {
              "type": "integer"
            },
            "changes": {
              "type": "integer"
            },
            "blob_url": {
              "type": "string",
              "format": "uri"
            },
            "raw_url": {
              "type": "string",
              "format": "uri"
            },
            "contents_url": {
              "type": "string",
              "format": "uri"
            },
            "patch": {
              "type": "string"
            },
            "previous_filename": {
              "type": "string"
            }
          },
          "required": [
            "additions",
            "blob_url",
            "changes",
            "contents_url",
            "deletions",
            "filename",
            "raw_url",
            "sha",
            "status"
          ]
        }
      }
    },
    "required": [
      "url",
      "sha",
      "node_id",
      "html_url",
      "comments_url",
      "commit",
      "author",
      "committer",
      "parents"
    ]
  }
```
## GitLab
### List repository commits
Get a list of repository commits in a project.

```plaintext
GET /projects/:id/repository/commits
```

#### Атрибуты запроса

| Attribute      | Type           | Required | Description |
|----------------|----------------|----------|-------------|
| `id`           | integer or string | Yes      | The ID or [URL-encoded path of the project](rest/_index.md#namespaced-paths). |
| `all`          | boolean        | No       | Retrieve every commit from the repository. If `true`, the `ref_name` parameter is ignored. |
| `author`       | string         | No       | Search commits by commit author. |
| `first_parent` | boolean        | No       | If `true`, follows only the first parent commit upon seeing a merge commit. |
| `follow`       | boolean        | No       | If `true`, follows file renames when filtering commits by `path`, and returns commits for the file even if it was renamed. If `false`, returns only commits where the file existed at its current path. Used only when `path` specifies a single file. Defaults to `true`. |
| `order`        | string         | No       | List commits in order. Possible values: `default`, [`topo`](https://git-scm.com/docs/git-log#Documentation/git-log.txt---topo-order). Defaults to `default`, the commits are shown in reverse chronological order. |
| `path`         | string         | No       | The file path. |
| `ref_name`     | string         | No       | The name of a repository branch, tag, or revision range, or if not given the default branch. |
| `since`        | string         | No       | Only commits after or on this date are returned in ISO 8601 format `YYYY-MM-DDTHH:MM:SSZ`. |
| `trailers`     | boolean        | No       | If `true`, parses and includes [Git trailers](https://git-scm.com/docs/git-interpret-trailers) for every commit. |
| `until`        | string         | No       | Only commits before or on this date are returned in ISO 8601 format `YYYY-MM-DDTHH:MM:SSZ`. |
| `with_stats`   | boolean        | No       | If `true`, retrieve stats about each commit. |

#### If successful, returns [`200 OK`](rest/troubleshooting.md#status-codes) and the following response attributes:

| Attribute           | Type   | Description |
|---------------------|--------|-------------|
| `author_email`      | string | Email address of the commit author. |
| `author_name`       | string | Name of the commit author. |
| `authored_date`     | string | Date when the commit was authored. |
| `committed_date`    | string | Date when the commit was committed. |
| `committer_email`   | string | Email address of the commit committer. |
| `committer_name`    | string | Name of the commit committer. |
| `created_at`        | string | Date when the commit was created (identical to `committed_date`). |
| `extended_trailers` | object | Extended Git trailers with all values. |
| `id`                | string | SHA of the commit. |
| `message`           | string | Full commit message. |
| `parent_ids`        | array  | Array of parent commit SHAs. |
| `short_id`          | string | Short SHA of the commit. |
| `title`             | string | Title of the commit message. |
| `trailers`          | object | Git trailers parsed from the commit message. |
| `web_url`           | string | Web URL of the commit. |
#### Пример запроса
```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/5/repository/commits"
```

#### Пример ответа:

```json
[
  {
    "id": "ed899a2f4b50b4370feeea94676502b42383c746",
    "short_id": "ed899a2f4b5",
    "title": "Replace sanitize with escape once",
    "author_name": "Example User",
    "author_email": "user@example.com",
    "authored_date": "2021-09-20T11:50:22.001+00:00",
    "committer_name": "Administrator",
    "committer_email": "admin@example.com",
    "committed_date": "2021-09-20T11:50:22.001+00:00",
    "created_at": "2021-09-20T11:50:22.001+00:00",
    "message": "Replace sanitize with escape once",
    "parent_ids": [
      "6104942438c14ec7bd21c6cd5bd995272b3faff6"
    ],
    "web_url": "https://gitlab.example.com/janedoe/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
    "trailers": {},
    "extended_trailers": {}
  },
  {
    "id": "6104942438c14ec7bd21c6cd5bd995272b3faff6",
    "short_id": "6104942438c",
    "title": "Sanitize for network graph",
    "author_name": "randx",
    "author_email": "user@example.com",
    "committer_name": "ExampleName",
    "committer_email": "user@example.com",
    "created_at": "2021-09-20T09:06:12.201+00:00",
    "message": "Sanitize for network graph\nCc: John Doe <johndoe@gitlab.com>\nCc: Jane Doe <janedoe@gitlab.com>",
    "parent_ids": [
      "ae1d9fb46aa2b07ee9836d49862ec4e2c46fbbba"
    ],
    "web_url": "https://gitlab.example.com/janedoe/gitlab-foss/-/commit/ed899a2f4b50b4370feeea94676502b42383c746",
    "trailers": {
      "Cc": "Jane Doe <janedoe@gitlab.com>"
    },
    "extended_trailers": {
      "Cc": [
        "John Doe <johndoe@gitlab.com>",
        "Jane Doe <janedoe@gitlab.com>"
      ]
    }
  }
]
```
# 7. Получение всех комментариев конкретного Pull Request и Merge Request

## GitHub

В GitHub комментарии к Pull Request разделены на несколько типов. Это связано с тем, что Pull Request также является Issue.

Для получения всех пользовательских комментариев конкретного Pull Request необходимо учитывать как минимум:

1. **Issue comments** — обычные комментарии в общей ленте Pull Request.
2. **Pull request review comments** — комментарии к конкретным строкам кода в diff Pull Request.

### List issue comments

Возвращает обычные комментарии конкретного Pull Request.

Так как каждый Pull Request в GitHub также является Issue, для получения таких комментариев используется endpoint из Issues API.

#### Endpoint

```http
GET /repos/{owner}/{repo}/issues/{issue_number}/comments
```

В случае Pull Request в качестве `issue_number` передается номер Pull Request.

> [!example]
> Для Pull Request `#42`:
> ```http
> GET /repos/octocat/Hello-World/issues/42/comments
> ```

#### Fine-grained access tokens

Endpoint поддерживает:

- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens

Fine-grained token должен иметь как минимум одно из разрешений:

- `Issues` repository permissions — `read`
- `Pull requests` repository permissions — `read`

Для публичных ресурсов endpoint может использоваться без аутентификации.

#### Параметры запроса

##### Headers

- `accept` string  
  Рекомендуется использовать `application/vnd.github+json`.

##### Path параметры

- `owner` string ==Необходимый==  
  Владелец репозитория.

- `repo` string ==Необходимый==  
  Название репозитория без `.git`.

- `issue_number` integer ==Необходимый==  
  Номер Issue или Pull Request.

##### Query параметры

- `since` string  
  Возвращает только комментарии, которые были обновлены после указанного времени. Используется формат ISO 8601:
  `YYYY-MM-DDTHH:MM:SSZ`.

- `per_page` integer  
  Количество результатов на одной странице. Максимальное значение — `100`.  
  По умолчанию — `30`.

- `page` integer  
  Номер страницы.  
  По умолчанию — `1`.

#### HTTP response status codes

| Status code | Объяснение |
| --- | --- |
| `200` | OK |
| `404` | Resource not found |
| `410` | Gone |

#### Пример запроса

```http
GET /repos/{owner}/{repo}/issues/{issue_number}/comments
```

```http
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/issues/ISSUE_NUMBER/comments
```

#### Основные данные ответа

Каждый элемент массива представляет отдельный комментарий. Среди наиболее важных полей:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | integer | Уникальный ID комментария |
| `[].node_id` | string | GraphQL Node ID комментария |
| `[].body` | string | Текст комментария |
| `[].user` | object | Пользователь, оставивший комментарий |
| `[].user.login` | string | Login автора |
| `[].created_at` | datetime | Время создания |
| `[].updated_at` | datetime | Время последнего изменения |
| `[].html_url` | string | URL комментария в GitHub |
| `[].issue_url` | string | API URL Issue/Pull Request |
| `[].author_association` | string | Отношение автора комментария к репозиторию |

---

### List review comments on a Pull Request

Возвращает review comments конкретного Pull Request.

Review comment отличается от обычного комментария тем, что он относится к review кода и может быть привязан к конкретному файлу и строке в diff.

#### Endpoint

```http
GET /repos/{owner}/{repo}/pulls/{pull_number}/comments
```

> [!important]
> Endpoint `/issues/{issue_number}/comments` и `/pulls/{pull_number}/comments` возвращают разные типы комментариев.
>
> Для получения полной картины комментариев Pull Request необходимо использовать оба endpoint.

#### Path параметры

- `owner` string ==Необходимый==  
  Владелец репозитория.

- `repo` string ==Необходимый==  
  Название репозитория без `.git`.

- `pull_number` integer ==Необходимый==  
  Номер Pull Request.

#### Основные данные ответа

Review comment содержит не только текст и автора, но также информацию о месте комментария в изменениях:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | integer | ID review comment |
| `[].pull_request_review_id` | integer | ID review, к которому относится комментарий |
| `[].body` | string | Текст комментария |
| `[].user` | object | Автор |
| `[].diff_hunk` | string | Часть diff, к которой относится комментарий |
| `[].path` | string | Путь к файлу |
| `[].commit_id` | string | SHA commit |
| `[].original_commit_id` | string | SHA исходного commit |
| `[].in_reply_to_id` | integer | ID комментария, на который является ответом данный комментарий |
| `[].created_at` | datetime | Время создания |
| `[].updated_at` | datetime | Время изменения |
| `[].line` | integer | Строка diff, к которой относится комментарий |
| `[].side` | string | Сторона diff (`LEFT` или `RIGHT`) |
| `[].start_line` | integer | Начальная строка многострочного комментария |
| `[].start_side` | string | Сторона начальной строки |

#### Пример запроса

```http
GET /repos/octocat/Hello-World/pulls/42/comments
```

Таким образом, для получения комментариев Pull Request в GitHub необходимо выполнить как минимум два запроса:

```text
GET /repos/{owner}/{repo}/issues/{pull_number}/comments
GET /repos/{owner}/{repo}/pulls/{pull_number}/comments
```

Первый возвращает обычные комментарии Pull Request, второй — комментарии code review.

---

## GitLab

В GitLab комментарии называются **Notes**.

Для конкретного Merge Request Notes API позволяет получить комментарии и системные записи, относящиеся к Merge Request.

### List all merge request notes

Возвращает все notes конкретного Merge Request.

#### Endpoint

```http
GET /projects/:id/merge_requests/:merge_request_iid/notes
```

#### Параметры запроса

##### Path параметры

- `id` integer или string ==Необходимый==  
  ID проекта или URL-encoded path проекта.

- `merge_request_iid` integer ==Необходимый==  
  Внутренний ID Merge Request в рамках проекта.

> [!important]
> Здесь используется именно `iid` Merge Request, а не глобальный `id`.

##### Query параметры

- `sort` string  
  Порядок сортировки.
  Возможные значения:
  - `asc`
  - `desc`

  По умолчанию — `asc`.

- `order_by` string  
  Поле, по которому сортируются результаты.

  Возможные значения:
  - `created_at`
  - `updated_at`

  По умолчанию — `created_at`.

#### Пример запроса

```http
GET /projects/:id/merge_requests/:merge_request_iid/notes
```

```http
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/5/merge_requests/11/notes"
```

#### Основные данные ответа

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | integer | ID note |
| `[].body` | string | Содержимое комментария |
| `[].author` | object | Автор |
| `[].author.id` | integer | ID автора |
| `[].author.username` | string | Username автора |
| `[].created_at` | datetime | Время создания |
| `[].updated_at` | datetime | Время последнего изменения |
| `[].system` | boolean | Является ли note системной записью |
| `[].noteable_id` | integer | ID объекта, к которому относится note |
| `[].noteable_type` | string | Тип объекта |
| `[].project_id` | integer | ID проекта |
| `[].resolvable` | boolean | Может ли note быть resolved |
| `[].confidential` | boolean | Является ли note конфиденциальной |
| `[].internal` | boolean | Является ли note внутренней |

> [!important]
> `system: true` означает, что запись была создана GitLab автоматически, а не является обычным пользовательским комментарием.

Например, изменение состояния, назначение пользователя или другие действия над Merge Request могут создавать system notes.

---

### Discussions

Если для RepoPulse необходимо получить не просто список notes, а сохранить структуру обсуждений и цепочки ответов, следует использовать **Discussions API**.

#### Endpoint

```http
GET /projects/:id/merge_requests/:merge_request_iid/discussions
```

Он возвращает discussions Merge Request. Каждая discussion содержит массив `notes`.

Основные поля:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | string | ID discussion |
| `[].individual_note` | boolean | Является ли discussion одиночной note |
| `[].notes` | array | Notes, входящие в discussion |
| `[].notes[].id` | integer | ID note |
| `[].notes[].type` | string | Тип note (`DiscussionNote`, `DiffNote` или `null`) |
| `[].notes[].body` | string | Текст |
| `[].notes[].author` | object | Автор |
| `[].notes[].created_at` | datetime | Время создания |

# 8. Получение всех issues и комментариев к ним
>[!note]
>Комментарии к issues расписаны в прошлом параграфе, так как pull request является сам по себе issue


## GitHub
### List repository issues
Выводит по дефолту открытые issues репозитория. 
#### Медиа форматы
Данный endpoint поддерживает несколько кастомных медиа форматов:
- application/vnd.github.raw+json: возвращает сырой markdown body. Ответ будет содержать body. Данный формат стоит по умолчанию, если не передать другой специфичный медиа формат.
- application/vnd.github.text+json: возвращает текст markdown body. Ответ будет содержать  body_text.
- application/vnd.github.html+json: возвращает HTML формат из  body's markdown. Ответ будет содержать body_html.
- application/vnd.github.full+json: Возвращает сырое, текстовое, и HTML представление. Ответ будет содержать body, body_text, and body_html.
#### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.
#### Параметры для запроса
##### Headers
- `accept` string
	Ставить `application/vnb.github+json` рекомендуется
##### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.
##### Query параметры
- `milestone` string
	Если передан `integer`, то он должен указывать на milestone по полю `number`. Если передана строка `*`, issues с любым milestone принимаются. Если передана строка `none`, то issues без milestone возвращаются
- `state` string
	Какие issues возвращать по состояния
	По умолчанию: `open`
	Другие значения: `open`, `closed`, `all`
- `assigne` string
	Может быть именем user. Ставить в `none` для issues, которые не имеют assigned users, и `*` если вообще все равно подписанные или нет.
- `type` string
	Может быть названием типа issue. Если передана `*`, то issues любого типа будут возвращены. Если `none`, то возвращаются только issues без типа.
- `creator` string
	Пользователь, который создал issue.
- `mentioned` string
	Пользователь, который упомянут в issue
- `issue_field_values` string
	Короче очень узкая настройка, нам не нужна
- `labels` string
	Список разделенных запятой заголовков. К примеру: `bug,ui,@high`
- `sort` string
	Как сортировать результат
	По умолчанию: `created`
	Может быть: `created`, `updated`, `comments`
- `direction` string
	Направление сортировки
	По умолчанию: `desc`
	Может быть: `asc`, `desc`
- `since` string
	Только показывает результат, который был обновлен после данного времени. Timestamp в формате ISO 8601
- `per_page` integer
	Сколько результатов на страницу (max 100).
- `page` integer
	Номер страницы для  получения результата. 
	По умолчанию: 1.

#### HTTP response status codes

| Status code | Description                                          |
| ----------- | ---------------------------------------------------- |
| 200         | OK                                                   |
| 301         | Moved permanently                                    |
| 404         | Resource not found                                   |
| 422         | Validation failed, or the endpoint has been spammed. |

#### Пример запроса
```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/issues
```


## GitLab
### List all issues
Lists all issues the authenticated user has access to. By default, returns only issues created by the current user. To list all issues, use parameter `scope=all`.

```plaintext
GET /issues
GET /issues?assignee_id=5
GET /issues?author_id=5
GET /issues?confidential=true
GET /issues?iids[]=42&iids[]=43
GET /issues?labels=foo
GET /issues?labels=foo,bar
GET /issues?labels=foo,bar&state=opened
GET /issues?milestone=1.0.0
GET /issues?milestone=1.0.0&state=opened
GET /issues?my_reaction_emoji=star
GET /issues?search=foo&in=title
GET /issues?state=closed
GET /issues?state=opened
```

#### Поддерживаемые атрибуты

| Attribute                       | Type          | Required   | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
|---------------------------------|---------------| ---------- |------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `assignee_id`                   | integer       | No         | Return issues assigned to the given user `id`. Mutually exclusive with `assignee_username`. `None` returns unassigned issues. `Any` returns issues with an assignee.                                                                                                                                                                                                                                                                                                                                                                                                   |
| `assignee_username`             | string array  | No         | Return issues assigned to the given `username`. Similar to `assignee_id` and mutually exclusive with `assignee_id`. In GitLab CE, the `assignee_username` array should only contain a single value. Otherwise, an invalid parameter error is returned. Only issues assigned to all passed users are returned. |
| `author_id`                     | integer       | No         | Return issues created by the given user `id`. Mutually exclusive with `author_username`. Combine with `scope=all` or `scope=assigned_to_me`.                                                                                                                                                                                                                                                                                                                                                                                                                           |
| `author_username`               | string        | No         | Return issues created by the given `username`. Similar to `author_id` and mutually exclusive with `author_id`.                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `confidential`                  | boolean       | No         | Filter confidential or public issues.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `created_after`                 | datetime      | No         | Return issues created on or after the given time. Expected in ISO 8601 format (`2019-03-15T08:00:00Z`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `created_before`                | datetime      | No         | Return issues created on or before the given time. Expected in ISO 8601 format (`2019-03-15T08:00:00Z`).                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `due_date`                      | string        | No         | Return issues that have no due date, are overdue, or whose due date is this week, this month, or between two weeks ago and next month. Accepts: `0` (no due date), `any`, `today`, `tomorrow`, `overdue`, `week`, `month`, `next_month_and_previous_two_weeks`.                                                                                                                                                                                                                                                                                                        |
| `epic_id`        | integer       | No         | Return issues associated with the given epic ID. `None` returns issues that are not associated with an epic. `Any` returns issues that are associated with an epic. Premium and Ultimate only.                                                                                                                                                                                                                                                                                                                                                                         |
| `health_status`  | string        | No         | Return issues with the specified `health_status`. `None` returns issues with no health status assigned, and `Any` returns issues with a health status assigned. Ultimate only.                                                                                                                                                                                                                |
| `iids[]`                        | integer array | No         | Return only the issues having the given `iid`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `in`                            | string        | No         | Modify the scope of the `search` attribute. `title`, `description`, or a string joining them with comma. Default is `title,description`.                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `issue_type`                    | string        | No         | Filter to a given type of issue. One of `issue`, `incident`, `test_case` or `task`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `iteration_id`                  | integer       | No         | Return issues assigned to the given iteration ID. `None` returns issues that do not belong to an iteration. `Any` returns issues that belong to an iteration. Mutually exclusive with `iteration_title`. Premium and Ultimate only.                                                                                                                                                                                                                                                                                                                                    |
| `iteration_title`               | string        | No       | Return issues assigned to the iteration with the given title. Similar to `iteration_id` and mutually exclusive with `iteration_id`. Premium and Ultimate only.                                                                                                                                                                                                                                                                                                                                                                                                         |
| `labels`                        | string        | No         | Comma-separated list of label names, issues must have all labels to be returned. `None` lists all issues with no labels. `Any` lists all issues with at least one label. `No+Label` (Deprecated) lists all issues with no labels. Predefined names are case-insensitive.                                                                                                                                                                                                                                                                                               |
| `milestone_id`                  | string        | No         | Returns issues assigned to milestones with a given timebox value (`None`, `Any`, `Upcoming`, and `Started`). `None` lists all issues with no milestone. `Any` lists all issues that have an assigned milestone. `Upcoming` lists all issues assigned to milestones due in the future. `Started` lists all issues assigned to open, started milestones. The logic for `Upcoming` and `Started` differs from the logic used in the [GraphQL API](../user/project/milestones/_index.md#special-milestone-filters). `milestone` and `milestone_id` are mutually exclusive. |
| `milestone`                     | string        | No         | The milestone title. `None` lists all issues with no milestone. `Any` lists all issues that have an assigned milestone. Using `None` or `Any` will be [deprecated in the future](https://gitlab.com/gitlab-org/gitlab/-/issues/336044). Use `milestone_id` attribute instead. `milestone` and `milestone_id` are mutually exclusive.                                                                                                                                                                                                                                   |
| `my_reaction_emoji`             | string        | No         | Return issues reacted by the authenticated user by the given `emoji`. `None` returns issues not given a reaction. `Any` returns issues given at least one reaction.                                                                                                                                                                                                                                                                                                                                                                                                    |
| `non_archived`                  | boolean       | No         | Return issues only from non-archived projects. If `false`, the response returns issues from both archived and non-archived projects. Default is `true`.                                                                                                                                                                                                                                                                                                                                                                                                                |
| `not`                           | Hash          | No         | Return issues that do not match the parameters supplied. Accepts: `assignee_id`, `assignee_username`, `author_id`, `author_username`, `iids`, `iteration_id`, `iteration_title`, `labels`, `milestone`, `milestone_id` and `weight`.                                                                                                                                                                                                                                                                                                                                   |
| `order_by`                      | string        | No         | Return issues ordered by `created_at`, `due_date`, `label_priority`, `milestone_due`, `popularity`, `priority`, `relative_position`, `title`, `updated_at`, or `weight` fields. Default is `created_at`.                                                                                                                                                                                                                                                                                                                                                               |
| `scope`                         | string        | No         | Return issues for the given scope: `created_by_me`, `assigned_to_me` or `all`. Defaults to `created_by_me`.                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `search`                        | string        | No         | Search issues against their `title` and `description`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `sort`                          | string        | No         | Return issues sorted in `asc` or `desc` order. Default is `desc`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `state`                         | string        | No         | Return `all` issues or just those that are `opened` or `closed`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `updated_after`                 | datetime      | No         | Return issues updated on or after the given time. Expected in ISO 8601 format (`2019-03-15T08:00:00Z`).                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `updated_before`                | datetime      | No         | Return issues updated on or before the given time. Expected in ISO 8601 format (`2019-03-15T08:00:00Z`).                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| `weight`                        | integer       | No         | Return issues with the specified `weight`. `None` returns issues with no weight assigned. `Any` returns issues with a weight assigned. Premium and Ultimate only.                                                                                                                                                                                                                                                                                                                                                                                                      |
| `with_labels_details`           | boolean       | No         | If `true`, the response returns more details for each label in the labels field: `:name`, `:color`, `:description`, `:description_html`, `:text_color`. Default is `false`.                                                                                                                                                                                                                                                                                                                                                                                                |

#### Пример запроса:

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/issues"
```

#### Пример ответа:

```json
[
   {
      "state" : "opened",
      "description" : "Ratione dolores corrupti mollitia soluta quia.",
      "author" : {
         "state" : "active",
         "id" : 18,
         "web_url" : "https://gitlab.example.com/eileen.lowe",
         "name" : "Alexandra Bashirian",
         "avatar_url" : null,
         "username" : "eileen.lowe"
      },
      "milestone" : {
         "project_id" : 1,
         "description" : "Ducimus nam enim ex consequatur cumque ratione.",
         "state" : "closed",
         "due_date" : null,
         "iid" : 2,
         "created_at" : "2016-01-04T15:31:39.996Z",
         "title" : "v4.0",
         "id" : 17,
         "updated_at" : "2016-01-04T15:31:39.996Z"
      },
      "project_id" : 1,
      "assignees" : [{
         "state" : "active",
         "id" : 1,
         "name" : "Administrator",
         "web_url" : "https://gitlab.example.com/root",
         "avatar_url" : null,
         "username" : "root"
      }],
      "assignee" : {
         "state" : "active",
         "id" : 1,
         "name" : "Administrator",
         "web_url" : "https://gitlab.example.com/root",
         "avatar_url" : null,
         "username" : "root"
      },
      "type" : "ISSUE",
      "updated_at" : "2016-01-04T15:31:51.081Z",
      "closed_at" : null,
      "closed_by" : null,
      "id" : 76,
      "title" : "Consequatur vero maxime deserunt laboriosam est voluptas dolorem.",
      "created_at" : "2016-01-04T15:31:51.081Z",
      "moved_to_id" : null,
      "iid" : 6,
      "labels" : ["foo", "bar"],
      "upvotes": 4,
      "downvotes": 0,
      "merge_requests_count": 0,
      "user_notes_count": 1,
      "start_date": null,
      "due_date": "2016-07-22",
      "imported":false,
      "imported_from": "none",
      "web_url": "http://gitlab.example.com/my-group/my-project/issues/6",
      "references": {
        "short": "#6",
        "relative": "my-group/my-project#6",
        "full": "my-group/my-project#6"
      },
      "time_stats": {
         "time_estimate": 0,
         "total_time_spent": 0,
         "human_time_estimate": null,
         "human_total_time_spent": null
      },
      "has_tasks": true,
      "task_status": "10 of 15 tasks completed",
      "confidential": false,
      "discussion_locked": false,
      "issue_type": "issue",
      "severity": "UNKNOWN",
      "_links":{
         "self":"http://gitlab.example.com/api/v4/projects/1/issues/76",
         "notes":"http://gitlab.example.com/api/v4/projects/1/issues/76/notes",
         "award_emoji":"http://gitlab.example.com/api/v4/projects/1/issues/76/award_emoji",
         "project":"http://gitlab.example.com/api/v4/projects/1",
         "closed_as_duplicate_of": "http://gitlab.example.com/api/v4/projects/1/issues/75"
      },
      "task_completion_status":{
         "count":0,
         "completed_count":0
      }
   }
]
```

# 9. Получение всех branches
## GitHub
### List branches
#### Fine-grained access tokens
Этот endpoint работает со следующими fine-grained access tokens
- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens
Но также этот endpoint может использоваться без аутентификации или конкретных разрешений в токене, если только публичные ресурсы запрашиваются.
#### Параметры для запроса
##### Headers
- `accept` string
	Ставить `application/vnb.github+json` рекомендуется
##### Path параметры
- `owner` string ==Необходимый==
	Указывается аккаунт владельца репозитория. Название не учитывает регистр.
- `repo` string ==Необходимый==
	Название репозитория без `.git` дополнения. Название не учитывает регистp.

#### Query параметры
- `protected` boolean
	Если поставить `true`, то возвращает только ветки, которые защищены правилом. Если установлено в `false`,  возвращает только незащищенные ветки. Если не ставить вообще этот параметр, то вернуться все ветки.
- `per_page` integer
	Сколько результатов на страницу (max 100).
- `page` integer
	Номер страницы для  получения результата. 
	По умолчанию: 1.

#### HTTP response status codes

| Status code | Description        |
| ----------- | ------------------ |
| 200         | OK                 |
| 404         | Resource not found |
#### Пример запроса
```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/branches
```
#### Пример ответа
```json
[
  {
    "name": "master",
    "commit": {
      "sha": "c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc",
      "url": "https://api.github.com/repos/octocat/Hello-World/commits/c5b97d5ae6c19d5c5df71a34c7fbeeda2479ccbc"
    },
    "protected": true,
    "protection": {
      "required_status_checks": {
        "enforcement_level": "non_admins",
        "contexts": [
          "ci-test",
          "linter"
        ]
      }
    },
    "protection_url": "https://api.github.com/repos/octocat/hello-world/branches/master/protection"
  }
]
```
## GitLab
### List all repository branches

Lists all repository branches from a project, sorted by name alphabetically. Search by name, or
use regular expressions to find specific branch patterns. Returns detailed information about the branch,
including its protection status, merge status, and commit details.

> [!note]
> This endpoint can be accessed without authentication if the repository is publicly accessible.

```plaintext
GET /projects/:id/repository/branches
```

#### Поддерживаемые атрибуты:

| Attribute | Type              | Required | Description |
|-----------|-------------------|----------|-------------|
| `id`      | integer or string | Yes      | ID or [URL-encoded path of the project](rest/_index.md#namespaced-paths). |
| `regex`   | string            | No       | Return list of branches with names matching a [re2](https://github.com/google/re2/wiki/Syntax) regular expression. Cannot be used together with `search`. |
| `search`  | string            | No       | Return list of branches containing the search string. You can use `^term` to find branches that begin with `term`, and `term$` to find branches that end with `term`. |

#### Если успешно, то вернется [`200 OK`](rest/troubleshooting.md#status-codes) со следующими атрибутами:

| Attribute                  | Type                | Description |
|----------------------------|---------------------|-------------|
| `can_push`                 | boolean             | If `true`, the authenticated user can push to this branch. |
| `commit`                   | object              | Details about the most recent commit on the branch. |
| `commit.author_email`      | string              | Email address of the user who authored the change. |
| `commit.author_name`       | string              | Name of the user who authored the change. |
| `commit.authored_date`     | datetime (ISO 8601) | When the commit was authored. |
| `commit.committed_date`    | datetime (ISO 8601) | When the commit was committed. |
| `commit.committer_email`   | string              | Email address of the user who committed the change. |
| `commit.committer_name`    | string              | Name of the user who committed the change. |
| `commit.created_at`        | datetime (ISO 8601) | When the commit was created. |
| `commit.extended_trailers` | object              | Extended Git trailers parsed from the commit message. |
| `commit.id`                | string              | Full SHA of the commit. |
| `commit.message`           | string              | Full commit message. |
| `commit.parent_ids`        | array               | Array of parent commit SHAs. |
| `commit.short_id`          | string              | Abbreviated SHA of the commit. |
| `commit.title`             | string              | Title of the commit message. |
| `commit.trailers`          | object              | Git trailers parsed from the commit message. |
| `commit.web_url`           | string              | URL to view the commit in the GitLab UI. |
| `default`                  | boolean             | If `true`, the branch is the default branch for the project. |
| `developers_can_merge`     | boolean             | If `true`, users with the Developer, Maintainer, or Owner role can merge to this branch. |
| `developers_can_push`      | boolean             | If `true`, users with the Developer, Maintainer, or Owner role can push to this branch. |
| `merged`                   | boolean             | If `true`, the branch has been merged into the default branch. |
| `name`                     | string              | Name of the branch. |
| `protected`                | boolean             | If `true`, the branch is protected from force pushes and deletion. |
| `web_url`                  | string              | URL to view the branch in the GitLab UI. |

#### Пример запроса:

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/5/repository/branches"
```

#### Пример ответа:

```json
[
  {
    "name": "main",
    "merged": false,
    "protected": true,
    "default": true,
    "developers_can_push": false,
    "developers_can_merge": false,
    "can_push": true,
    "web_url": "https://gitlab.example.com/my-group/my-project/-/tree/main",
    "commit": {
      "id": "7b5c3cc8be40ee161ae89a06bba6229da1032a0c",
      "short_id": "7b5c3cc",
      "created_at": "2024-06-28T03:44:20-07:00",
      "parent_ids": [
        "4ad91d3c1144c406e50c7b33bae684bd6837faf8"
      ],
      "title": "add projects API",
      "message": "add projects API",
      "author_name": "John Smith",
      "author_email": "john@example.com",
      "authored_date": "2024-06-27T05:51:39-07:00",
      "committer_name": "John Smith",
      "committer_email": "john@example.com",
      "committed_date": "2024-06-28T03:44:20-07:00",
      "trailers": {},
      "extended_trailers": {},
      "web_url": "https://gitlab.example.com/my-group/my-project/-/commit/7b5c3cc8be40ee161ae89a06bba6229da1032a0c"
    }
  },
  ...
]
```

# 10. Получение данных о Code Review и комментариях к изменениям

## GitHub

В GitHub Code Review для Pull Request представлен отдельной сущностью **Pull Request Review**.

Review описывает результат проверки Pull Request конкретным пользователем. В рамках Review пользователь может оставить общее сообщение, принять изменения (`APPROVED`), запросить изменения (`CHANGES_REQUESTED`), а также оставить комментарии непосредственно к строкам измененного кода.

Комментарии непосредственно к diff представлены отдельной сущностью — **Pull Request Review Comment**.

Таким образом, для получения информации о Code Review необходимо отдельно получать:

- Reviews конкретного Pull Request;
- Review Comments, оставленные непосредственно к изменениям.

### List reviews for a Pull Request

Возвращает все reviews конкретного Pull Request.

#### Endpoint

```http
GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews
```

#### Fine-grained access tokens

Endpoint поддерживает:

- GitHub App user access tokens;
- GitHub App installation access tokens;
- Fine-grained personal access tokens.

Fine-grained access token должен иметь:

```text
Pull requests repository permissions: read
```

Для публичных репозиториев endpoint может использоваться без аутентификации.

#### Параметры запроса

##### Headers

- `accept` string  
  Рекомендуется использовать:

```text
application/vnd.github+json
```

##### Path параметры

- `owner` string ==Необходимый==  
  Владелец репозитория.

- `repo` string ==Необходимый==  
  Название репозитория без `.git`.

- `pull_number` integer ==Необходимый==  
  Номер Pull Request.

##### Query параметры

- `per_page` integer  
  Количество результатов на одной странице. Максимальное значение — `100`.  
  По умолчанию — `30`.

- `page` integer  
  Номер страницы.  
  По умолчанию — `1`.

#### Пример запроса

```http
GET /repos/octocat/Hello-World/pulls/42/reviews
```

```http
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/pulls/PULL_NUMBER/reviews
```

#### Основные данные ответа

Каждый элемент массива представляет отдельный Review.

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | integer | Уникальный ID Review |
| `[].node_id` | string | GraphQL Node ID Review |
| `[].user` | object | Пользователь, выполнивший Review |
| `[].user.login` | string | Login reviewer |
| `[].body` | string | Текст Review |
| `[].state` | string | Состояние Review |
| `[].html_url` | string | URL Review |
| `[].pull_request_url` | string | API URL Pull Request |
| `[].author_association` | string | Отношение reviewer к репозиторию |
| `[].submitted_at` | datetime | Время отправки Review |
| `[].commit_id` | string | SHA commit, который проверялся |

Поле `state` позволяет определить результат Code Review.

Например:

```text
APPROVED
CHANGES_REQUESTED
COMMENTED
DISMISSED
PENDING
```

> [!important]
> Review относится к определенному состоянию Pull Request. Поле `commit_id` позволяет определить commit, относительно которого был выполнен Review.

---

### Получение комментариев к изменениям

### List review comments on a Pull Request

Review Comments — комментарии, оставленные непосредственно на части unified diff во время Code Review.

Они отличаются от обычных комментариев Pull Request.

Обычный комментарий:

```text
Pull Request
└── Comment: "Нужно добавить тесты"
```

Review Comment:

```text
Pull Request
└── internal/service/user.go
      │
      └── line 42
            └── "Здесь ошибка обработки context"
```

#### Endpoint

```http
GET /repos/{owner}/{repo}/pulls/{pull_number}/comments
```

#### Fine-grained access tokens

Необходимое разрешение:

```text
Pull requests repository permissions: read
```

Для публичных репозиториев endpoint может использоваться без аутентификации.

#### Параметры запроса

##### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `pull_number` integer ==Необходимый==

##### Query параметры

- `sort` string  
  Поле сортировки.

  Возможные значения:

```text
created
updated
created_at
```

- `direction` string  
  Направление сортировки:

```text
asc
desc
```

- `since` string  
  Возвращает комментарии, обновленные после указанного времени.

  Формат ISO 8601:

```text
YYYY-MM-DDTHH:MM:SSZ
```

- `per_page` integer  
  Количество результатов на странице. Максимум — `100`.

- `page` integer  
  Номер страницы.

#### Пример запроса

```http
GET /repos/octocat/Hello-World/pulls/42/comments
```

#### Основные данные ответа

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | integer | ID комментария |
| `[].pull_request_review_id` | integer | ID Review, в рамках которого был создан комментарий |
| `[].body` | string | Текст комментария |
| `[].user` | object | Автор комментария |
| `[].diff_hunk` | string | Часть diff, к которой относится комментарий |
| `[].path` | string | Путь измененного файла |
| `[].commit_id` | string | SHA commit |
| `[].original_commit_id` | string | Исходный SHA commit |
| `[].in_reply_to_id` | integer | ID родительского комментария, если это ответ |
| `[].created_at` | datetime | Время создания |
| `[].updated_at` | datetime | Время изменения |
| `[].start_line` | integer | Начальная строка многострочного комментария |
| `[].line` | integer | Строка, к которой относится комментарий |
| `[].start_side` | string | Сторона diff для начальной строки |
| `[].side` | string | Сторона diff |
| `[].original_line` | integer | Исходный номер строки |
| `[].original_start_line` | integer | Исходная начальная строка |

`side` может иметь значения:

```text
LEFT
RIGHT
```

`LEFT` означает удаляемую/старую сторону diff.

`RIGHT` означает добавляемую/новую сторону diff.

Поле `diff_hunk` позволяет получить участок diff, относительно которого был оставлен комментарий.

Например:

```json
{
  "pull_request_review_id": 42,
  "diff_hunk": "@@ -16,33 +16,40 @@ public class Connection...",
  "path": "file1.txt",
  "commit_id": "6dcb09b5b57875f334f61aebed695e2e4193db5e",
  "body": "Great stuff!",
  "start_line": 1,
  "line": 2,
  "side": "RIGHT"
}
```

Таким образом, по Review Comment можно определить:

```text
кто оставил комментарий
        ↓
в рамках какого Review
        ↓
к какому файлу
        ↓
к какому commit
        ↓
к какой части diff
        ↓
к какой строке
        ↓
что написал reviewer
```

---

### List comments for a Pull Request Review

Если уже известен конкретный `review_id`, GitHub позволяет получить комментарии именно этого Review.

#### Endpoint

```http
GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews/{review_id}/comments
```

#### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `pull_number` integer ==Необходимый==
- `review_id` integer ==Необходимый==

#### Query параметры

- `per_page` integer  
  Количество результатов на странице. Максимум — `100`.

- `page` integer  
  Номер страницы.

#### Пример запроса

```http
GET /repos/octocat/Hello-World/pulls/42/reviews/80/comments
```

> [!important]
> Для получения всех review comments Pull Request необязательно выполнять отдельный запрос для каждого Review.
>
> Endpoint:
>
> ```http
> GET /repos/{owner}/{repo}/pulls/{pull_number}/comments
> ```
>
> сразу возвращает review comments Pull Request.
>
> Поле `pull_request_review_id` позволяет связать каждый комментарий с соответствующим Review.

---

## GitLab

В GitLab модель Code Review отличается от GitHub.

GitHub имеет отдельную сущность `Pull Request Review`, содержащую состояние вроде `APPROVED` или `CHANGES_REQUESTED`.

В GitLab для получения аналогичной информации необходимо рассматривать несколько API:

- Merge Request Approvals API — информация об одобрении Merge Request;
- Discussions API — обсуждения и комментарии;
- Diff Notes — комментарии непосредственно к измененияенным строкам.

### Получение состояния Approval Merge Request

Для получения информации об одобрении конкретного Merge Request используется Merge Request Approvals API.

#### Endpoint

```http
GET /projects/:id/merge_requests/:merge_request_iid/approvals
```

> [!important]
> Для Merge Request используется `merge_request_iid`, то есть внутренний ID Merge Request внутри проекта.

#### Аутентификация

Endpoints Merge Request Approvals API требуют аутентификации.

#### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

- `merge_request_iid` integer ==Необходимый==  
  IID Merge Request.

#### Пример запроса

```http
GET /projects/5/merge_requests/11/approvals
```

#### Основные данные ответа

Среди важных данных:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `approved` | boolean | Выполнены ли требования к approvals |
| `approved_by` | array | Пользователи, одобрившие Merge Request |
| `approved_by[].user` | object | Пользователь, выполнивший approval |
| `approved_by[].approved_at` | datetime | Время approval |

`approved_by` содержит информацию о пользователях, которые одобрили Merge Request.

> [!important]
> Значение `approved` зависит от конфигурации GitLab и настроенных approval requirements.
>
> Поэтому `approved_by` и `approved` имеют разный смысл: первое показывает пользователей, выполнивших approval, а второе — удовлетворяет ли Merge Request требованиям approval.

---

### Получение комментариев к изменениям в GitLab

Для получения структуры Code Review и комментариев к конкретным изменениям используется **Discussions API**.

#### List all Merge Request discussion items

Возвращает все discussion items конкретного Merge Request.

##### Endpoint

```http
GET /projects/:id/merge_requests/:merge_request_iid/discussions
```

##### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

- `merge_request_iid` integer ==Необходимый==  
  IID Merge Request.

##### Пример запроса

```http
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/5/merge_requests/11/discussions"
```

##### Основные данные ответа

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `[].id` | string | ID discussion |
| `[].individual_note` | boolean | Является ли discussion отдельной note |
| `[].notes` | array | Notes внутри discussion |
| `[].notes[].id` | integer | ID note |
| `[].notes[].type` | string | Тип note |
| `[].notes[].body` | string | Текст комментария |
| `[].notes[].author` | object | Автор |
| `[].notes[].created_at` | datetime | Время создания |
| `[].notes[].updated_at` | datetime | Время изменения |
| `[].notes[].system` | boolean | Является ли note системной |
| `[].notes[].resolved` | boolean | Был ли комментарий resolved |
| `[].notes[].resolvable` | boolean | Может ли комментарий быть resolved |
| `[].notes[].resolved_by` | object | Кто разрешил discussion |
| `[].notes[].resolved_at` | datetime | Когда discussion был resolved |
| `[].notes[].position` | object | Позиция комментария относительно diff |
| `[].notes[].suggestions` | array | Suggestions, содержащиеся в note |

Поле `type` может содержать:

```text
DiscussionNote
DiffNote
null
```

Для анализа комментариев непосредственно к изменениям особенно важен:

```text
DiffNote
```

---

#### Position DiffNote

Если note является комментарием к изменению кода, она содержит объект `position`.

Position позволяет определить, к какому месту diff относится комментарий.

В нем могут находиться:

```text
base_sha
start_sha
head_sha

old_path
new_path

old_line
new_line

position_type
```

SHA используются для определения версии diff:

- `base_sha` — base commit;
- `head_sha` — HEAD commit Merge Request;
- `start_sha` — commit target branch, относительно которого началось сравнение.

Пути определяют изменяемый файл:

```text
old_path
new_path
```

А строки:

```text
old_line
new_line
```

позволяют определить конкретное изменение.

Например концептуально:

```json
{
  "type": "DiffNote",
  "body": "Здесь необходимо обработать ошибку",
  "position": {
    "base_sha": "...",
    "start_sha": "...",
    "head_sha": "...",
    "old_path": "internal/service.go",
    "new_path": "internal/service.go",
    "old_line": null,
    "new_line": 42
  }
}
```

---


# 11. Получение данных о workflow, pipeline и статусах CI/CD

## GitHub

Для работы с CI/CD в GitHub используется GitHub Actions.

Через REST API можно получить информацию о:

- workflows репозитория;
- запусках workflow (workflow runs);
- jobs конкретного workflow run;
- статусах commit.

### Получение Workflows репозитория

#### List repository workflows

Возвращает workflows в конкретном репозитории.

##### Fine-grained access tokens

Данный endpoint работает со следующими fine-grained access tokens:

- GitHub App user access tokens
- GitHub App installation access tokens
- Fine-grained personal access tokens

Fine-grained token должен иметь следующее разрешение:

- `Actions` repository permissions — `read`

Endpoint может использоваться без аутентификации или указанных разрешений, если запрашиваются только публичные ресурсы.

##### Параметры запроса

##### Headers

- `accept` string  
  Рекомендуется использовать `application/vnd.github+json`.

##### Path параметры

- `owner` string ==Необходимый==  
  Владелец репозитория. Название не учитывает регистр.

- `repo` string ==Необходимый==  
  Название репозитория без `.git`. Название не учитывает регистр.

##### Query параметры

- `per_page` integer  
  Количество результатов на одной странице. Максимальное значение — `100`.  
  По умолчанию: `30`.

- `page` integer  
  Номер страницы.  
  По умолчанию: `1`.

##### HTTP response status codes

| Status code | Объяснение |
| --- | --- |
| `200` | OK |

##### Пример запроса

```http id="pr3v1n"
GET /repos/{owner}/{repo}/actions/workflows
```

```http id="l1h47x"
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer <YOUR-TOKEN>" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/OWNER/REPO/actions/workflows
```

##### Пример ответа

```json id="1sv2o2"
{
  "total_count": 2,
  "workflows": [
    {
      "id": 161335,
      "node_id": "MDg6V29ya2Zsb3cxNjEzMzU=",
      "name": "CI",
      "path": ".github/workflows/blank.yaml",
      "state": "active",
      "created_at": "2020-01-08T23:48:37.000-08:00",
      "updated_at": "2020-01-08T23:50:21.000-08:00",
      "url": "https://api.github.com/repos/octo-org/octo-repo/actions/workflows/161335",
      "html_url": "https://github.com/octo-org/octo-repo/blob/master/.github/workflows/161335",
      "badge_url": "https://github.com/octo-org/octo-repo/workflows/CI/badge.svg"
    }
  ]
}
```

##### Основные данные ответа

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `total_count` | integer | Количество workflows |
| `workflows[].id` | integer | ID workflow |
| `workflows[].name` | string | Название workflow |
| `workflows[].path` | string | Путь к файлу workflow |
| `workflows[].state` | string | Состояние workflow |
| `workflows[].created_at` | datetime | Время создания |
| `workflows[].updated_at` | datetime | Время обновления |
| `workflows[].url` | string | API URL workflow |
| `workflows[].html_url` | string | URL workflow в GitHub |
| `workflows[].badge_url` | string | URL status badge workflow |

---

### Получение конкретного Workflow

#### Get a workflow

Возвращает информацию о конкретном workflow.

#### Endpoint

```http id="n1d19r"
GET /repos/{owner}/{repo}/actions/workflows/{workflow_id}
```

`workflow_id` может быть ID workflow или именем файла workflow.

Например:

```text id="tzqaf6"
main.yaml
```

#### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `workflow_id` integer или string ==Необходимый==  
  ID workflow или имя файла workflow.

#### HTTP response status codes

| Status code | Объяснение |
| --- | --- |
| `200` | OK |

#### Пример запроса

```http id="25pds5"
GET /repos/octo-org/octo-repo/actions/workflows/161335
```

---

### Получение запусков Workflow

Workflow сам по себе описывает workflow GitHub Actions. Конкретный запуск workflow представлен сущностью **Workflow Run**.

Workflow Run создается, когда workflow запускается в результате настроенного события.

#### List workflow runs for a workflow

Возвращает workflow runs конкретного workflow.

##### Endpoint

```http id="92t7hp"
GET /repos/{owner}/{repo}/actions/workflows/{workflow_id}/runs
```

##### Fine-grained access tokens

Fine-grained token должен иметь:

- `Actions` repository permissions — `read`

Для публичных ресурсов endpoint может использоваться без аутентификации.

##### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `workflow_id` integer или string ==Необходимый==

##### Query параметры

Endpoint поддерживает фильтрацию workflow runs, в том числе по:

- `actor`
- `branch`
- `event`
- `status`
- `created`
- `head_sha`

Также поддерживается pagination:

- `per_page`
- `page`

`per_page` имеет максимальное значение `100`.

##### Пример запроса

```http id="n3tk7j"
GET /repos/octo-org/octo-repo/actions/workflows/161335/runs
```

---

#### List workflow runs for a repository

Также можно получить workflow runs всего репозитория, не указывая конкретный workflow.

##### Endpoint

```http id="idjnsh"
GET /repos/{owner}/{repo}/actions/runs
```

Этот endpoint позволяет получить runs различных workflows репозитория.

##### Основные данные Workflow Run

Среди данных workflow run можно получить:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID workflow run |
| `name` | string | Название workflow |
| `node_id` | string | GraphQL Node ID |
| `head_branch` | string | Ветка, относительно которой запущен workflow |
| `head_sha` | string | SHA commit |
| `path` | string | Путь workflow |
| `run_number` | integer | Номер запуска |
| `event` | string | Событие, вызвавшее запуск |
| `status` | string | Текущий статус workflow run |
| `conclusion` | string или null | Результат завершенного workflow run |
| `workflow_id` | integer | ID workflow |
| `url` | string | API URL workflow run |
| `html_url` | string | URL workflow run в GitHub |
| `created_at` | datetime | Время создания |
| `updated_at` | datetime | Время последнего обновления |
| `run_started_at` | datetime | Время начала выполнения |
| `jobs_url` | string | URL для получения jobs |
| `logs_url` | string | URL для получения logs |
| `check_suite_url` | string | URL связанного check suite |

> [!important]
> `status` и `conclusion` являются разными полями.
>
> `status` показывает состояние выполнения workflow run, а `conclusion` — результат завершенного выполнения.

---

### Получение Jobs Workflow Run

Workflow Run может состоять из нескольких jobs.

Workflow Job представляет набор steps, которые выполняются на одном runner.

#### List jobs for a workflow run

##### Endpoint

```http id="k2v50d"
GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs
```

##### Fine-grained access tokens

Fine-grained token должен иметь:

- `Actions` repository permissions — `read`

Для публичных ресурсов endpoint может использоваться без аутентификации.

##### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `run_id` integer ==Необходимый==  
  ID workflow run.

##### Query параметры

- `filter` string  
  Фильтрует jobs.

- `per_page` integer  
  Количество результатов на странице. Максимум `100`.

- `page` integer  
  Номер страницы.

##### Основные данные Job

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID job |
| `run_id` | integer | ID workflow run |
| `workflow_name` | string | Название workflow |
| `head_sha` | string | SHA commit |
| `status` | string | Статус job |
| `conclusion` | string или null | Результат выполнения job |
| `started_at` | datetime | Время начала |
| `completed_at` | datetime | Время завершения |
| `name` | string | Название job |
| `steps` | array | Steps, входящие в job |
| `runner_id` | integer | ID runner |
| `runner_name` | string | Название runner |
| `runner_group_id` | integer | ID группы runner |
| `runner_group_name` | string | Название группы runner |

##### Steps

Внутри job можно получить информацию об отдельных steps.

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `steps[].name` | string | Название step |
| `steps[].status` | string | Статус step |
| `steps[].conclusion` | string или null | Результат step |
| `steps[].number` | integer | Номер step |
| `steps[].started_at` | datetime | Время начала |
| `steps[].completed_at` | datetime | Время завершения |

Таким образом структура GitHub Actions через REST API выглядит следующим образом:

```text id="mlpn6j"
Workflow
    │
    └── Workflow Run
            │
            ├── Job
            │    ├── Step
            │    ├── Step
            │    └── Step
            │
            └── Job
                 ├── Step
                 └── Step
```

---

### Получение CI/CD статусов commit

GitHub также предоставляет отдельный Commit Status API.

Commit statuses позволяют внешним сервисам устанавливать status для commit. Например, CI сервис может пометить commit в зависимости от результата build.

Статус может иметь следующие состояния:

```text id="0f5jtz"
error
failure
pending
success
```

#### List commit statuses for a reference

Возвращает commit statuses для указанного Git reference.

##### Endpoint

```http id="p0xtp5"
GET /repos/{owner}/{repo}/commits/{ref}/statuses
```

`ref` может представлять:

- commit SHA;
- branch;
- tag.

##### Fine-grained access tokens

Fine-grained token должен иметь:

- `Commit statuses` repository permissions — `read`

Для публичных ресурсов endpoint может использоваться без аутентификации.

##### Path параметры

- `owner` string ==Необходимый==
- `repo` string ==Необходимый==
- `ref` string ==Необходимый==  
  Commit SHA, branch или tag.

##### Query параметры

- `per_page` integer  
  Максимум `100`.

- `page` integer

##### Основные данные ответа

Commit status содержит:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `state` | string | Состояние status |
| `description` | string или null | Описание status |
| `target_url` | string или null | URL системы, предоставившей status |
| `context` | string | Название/контекст системы, предоставившей status |
| `created_at` | datetime | Время создания |
| `updated_at` | datetime | Время обновления |
| `creator` | object | Пользователь, создавший status |

Поле `context` позволяет различать statuses от разных систем.

Например:

```text id="8k0xma"
ci
security
```

---

#### Get the combined status for a specific reference

Позволяет получить объединенное состояние commit statuses для конкретного ref.

##### Endpoint

```http id="egb2of"
GET /repos/{owner}/{repo}/commits/{ref}/status
```

В ответе возвращается объединенное поле `state`.

Оно определяется следующим образом:

- `failure` — если хотя бы один context имеет `error` или `failure`;
- `pending` — если statuses отсутствуют или хотя бы один context имеет `pending`;
- `success` — если последние statuses всех contexts имеют `success`.

> [!important]
> Commit Status API и GitHub Actions Workflow Runs API являются разными API.
>
> Для получения информации непосредственно о GitHub Actions необходимо использовать Actions API.
>
> Commit Status API используется для работы со statuses конкретного commit/ref.

---

## GitLab

Для CI/CD в GitLab используется сущность **Pipeline**.

Для получения данных о CI/CD через REST API используются:

- Pipelines API;
- Jobs API;
- Commit Status API.

### Получение Pipelines проекта

#### List project pipelines

Возвращает pipelines конкретного проекта.

#### Endpoint

```http id="5i4qzz"
GET /projects/:id/pipelines
```

#### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

#### Query параметры

Endpoint позволяет фильтровать pipelines по следующим параметрам:

- `id`
- `name`
- `ref`
- `sha`
- `source`
- `status`
- `updated_after`
- `updated_before`
- `username`
- `yaml_errors`

Также можно задавать:

- `order_by`
- `sort`

#### Status pipeline

Параметр `status` позволяет фильтровать pipelines по status.

GitLab документация указывает следующие значения:

```text id="3f2l4n"
created
waiting_for_resource
preparing
pending
running
success
failed
canceled
skipped
manual
scheduled
```

#### Пример запроса

```http id="ocq57q"
GET /projects/1/pipelines
```

```http id="rbj4ts"
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/1/pipelines"
```

#### Основные данные Pipeline

Среди возвращаемых данных:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID pipeline |
| `iid` | integer | IID pipeline |
| `project_id` | integer | ID проекта |
| `sha` | string | SHA commit |
| `ref` | string | Git ref |
| `status` | string | Status pipeline |
| `source` | string | Источник запуска pipeline |
| `created_at` | datetime | Время создания |
| `updated_at` | datetime | Время обновления |
| `web_url` | string | URL pipeline |
| `name` | string | Название pipeline |

---

### Получение конкретного Pipeline

#### Get a single pipeline

Возвращает информацию о конкретном pipeline.

#### Endpoint

```http id="a6yx4i"
GET /projects/:id/pipelines/:pipeline_id
```

#### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

- `pipeline_id` integer ==Необходимый==  
  ID pipeline.

#### Пример запроса

```http id="57cz7x"
GET /projects/1/pipelines/46
```

Ответ конкретного pipeline содержит дополнительные данные, включая информацию о времени выполнения.

Среди возвращаемых полей:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID pipeline |
| `iid` | integer | IID pipeline |
| `project_id` | integer | ID проекта |
| `sha` | string | SHA commit |
| `ref` | string | Git ref |
| `status` | string | Status pipeline |
| `source` | string | Источник pipeline |
| `created_at` | datetime | Время создания |
| `updated_at` | datetime | Время обновления |
| `started_at` | datetime | Время начала |
| `finished_at` | datetime | Время завершения |
| `duration` | integer | Продолжительность pipeline |
| `queued_duration` | integer | Время ожидания pipeline |
| `coverage` | string или null | Code coverage |
| `web_url` | string | URL pipeline |
| `user` | object | Пользователь, связанный с pipeline |

---

### Получение Jobs Pipeline

#### List all jobs by pipeline

Возвращает jobs конкретного pipeline.

##### Endpoint

```http id="a9fh5e"
GET /projects/:id/pipelines/:pipeline_id/jobs
```

##### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

- `pipeline_id` integer ==Необходимый==  
  ID pipeline.

##### Query параметры

- `include_retried` boolean  
  Включить retried jobs в результат.  
  По умолчанию: `false`.

- `scope` string или array of strings  
  Фильтрует jobs по status.

Если `scope` не передан, возвращаются jobs независимо от status.

> [!important]
> Endpoint возвращает данные для любого pipeline, включая child pipelines.
>
> По умолчанию retried jobs в ответ не включаются.
>
> Jobs сортируются по ID в порядке убывания.

##### Пример запроса

```http id="ewtn8r"
GET /projects/1/pipelines/46/jobs
```

##### Основные данные Job

Среди данных job:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID job |
| `status` | string | Status job |
| `stage` | string | Stage, в котором выполняется job |
| `name` | string | Название job |
| `ref` | string | Git ref |
| `created_at` | datetime | Время создания |
| `started_at` | datetime или null | Время начала |
| `finished_at` | datetime или null | Время завершения |
| `duration` | number или null | Продолжительность |
| `queued_duration` | number или null | Время ожидания |
| `allow_failure` | boolean | Разрешено ли job завершиться с ошибкой |
| `web_url` | string | URL job |
| `user` | object | Пользователь |
| `commit` | object | Commit |
| `pipeline` | object | Pipeline |
| `runner` | object или null | Runner |
| `artifacts` | array | Artifacts job |

GitLab Jobs API использует следующие status values:

```text id="vb8t4s"
created
waiting_for_resource
preparing
pending
running
success
failed
canceling
canceled
skipped
manual
scheduled
```

Таким образом основная структура GitLab CI/CD:

```text id="p33f79"
Pipeline
    │
    ├── Job
    │
    ├── Job
    │
    └── Job
```

---

### Получение Commit Status в GitLab

GitLab предоставляет Commit Status API.

#### List commit statuses

Возвращает statuses указанного commit проекта.

##### Endpoint

```http id="1ms4kn"
GET /projects/:id/repository/commits/:sha/statuses
```

#### Path параметры

- `id` integer или string ==Необходимый==  
  ID или URL-encoded path проекта.

- `sha` string ==Необходимый==  
  Hash commit.

#### Query параметры

- `all` boolean  
  Если `true`, возвращаются все statuses, а не только последние.  
  По умолчанию: `false`.

- `name` string  
  Фильтрация statuses по имени job.

Например:

```text id="3vpyw7"
bundler:audit
```

- `order_by` string  
  Сортировка statuses.

Возможные значения:

```text id="fdoxl4"
id
pipeline_id
```

По умолчанию: `id`.

- `pipeline_id` integer  
  Фильтрация statuses по ID pipeline.

- `ref` string  
  Имя branch или tag.

- `sort` string  
  Направление сортировки.

- `stage` string  
  Фильтрация по stage.

Также endpoint поддерживает pagination через:

- `page`
- `per_page`

#### Пример запроса

```http id="l1oj3z"
GET /projects/17/repository/commits/6b2257d/statuses
```

#### Данные Commit Status

Ответ позволяет получить данные status, связанные с commit, в том числе:

| Атрибут | Тип | Объяснение |
| --- | --- | --- |
| `id` | integer | ID status |
| `sha` | string | SHA commit |
| `ref` | string | Git ref |
| `status` | string | Status |
| `name` | string | Название status/job |
| `target_url` | string | URL, связанный со status |
| `description` | string | Описание |
| `created_at` | datetime | Время создания |
| `started_at` | datetime | Время начала |
| `finished_at` | datetime | Время завершения |
| `allow_failure` | boolean | Разрешено ли завершение с ошибкой |
| `pipeline_id` | integer | ID связанного pipeline |

---


# 12 .Ограничения API
## GitHub
### Rate Limits
- Если вы не аутентифицированный пользователь, то rate limit равен 60 запросам в час.
- Если аутентификация пройдена, то количество запросов в час возрастает до 5000.
Есть также вторичные лимиты:
- Не более чем 100 конкурентных запросов
- Не более 900 запросов в минуту в один endpoint
- Делать слишком много запросов в минуту. Не более чем 90 секунд CPU времени каждые 60 секунд реального времени. 
Также можно посылать некоторые headers вместе с запросами, чтобы узнать свои primary rate limits.

| Header name             | Description                                                                      |
| ----------------------- | -------------------------------------------------------------------------------- |
| `x-ratelimit-limt`      | Максимальное количество запросов, которое пользователь может делать в час        |
| `x-ratelimit-remaining` | Количество оставшихся запросов нашего rate limit промежутка                      |
| `x-ratelimit-used`      | Количество запросов, которые были сделаны в промежуток rate limit                |
| `x-ratelimit-reset`     | Время, когда наш rate limit обнулится. Приходит в UTC epoch seconds              |
| `x-ratelimit-resource`  | ресурс ограничения частоты запросов, в рамках которого учитывался данный запрос. |
|                         |                                                                                  |
### Пагинация
Если ответ должен содержать много результатов, GitHub осуществит пагинацию и вернет срез резульаттов. 
Для того, чтобы достать другие страницы, можно использовать header `link` из ответа на наш запрос. Если запрос поддерживает `per_page`, то мы можем контролировать количество результатов на странице.
#### Using `link` headers
Когда ответ пагинируется, этот ответ будет содержать в себе header `link`. Если endpoint не поддерживает пагинацию, или все результаты поместятся на одной станице, то header `link` будет отсутствовать.
`link` header содержит в себе URLs, которые можно использовать, для получения недостающих страниц.
Если endpoint поддерживает пагинацию, ответ будет выглядеть так:
```http
link: <https://api.github.com/repositories/1300192/issues?page=2>; rel="prev", <https://api.github.com/repositories/1300192/issues?page=4>; rel="next", <https://api.github.com/repositories/1300192/issues?page=515>; rel="last", <https://api.github.com/repositories/1300192/issues?page=1>; rel="first"
```

- `rel="prev"` это URl на предыдущую страницу
- `rel="next"` это URL на следующую страницу
- `rel="last"` это URL на последнюю страницу
- `rel="first"` это URL на первую страницу

## GitLab
### Rate limits
Разделяется на два лимита: Sustained limits и Burst limits.
**Sustained limit** - основной часовой лимит.
- Для авторизованного пользователя на бесплатном тарифе лимит составляет 5000 запросов в час
- Для не авторизованного пользователя на бесплатном тарифе лимит составляет 60 запросов в час
**Burst limits** - это лимит, обновляемый каждую минуту, чтобы всплеск запросов не израсходовал часовой лимит за 1 минуту.
- Для авторизованного пользователя лимит составляет 100 запросов в минуту
>[!warning]
>Когда лимит достигнут, GitLab будет отвечать `429 Too Many Requests`.

Ответы ограниченных запросов содержит в себе `Retry-After` header, который говорит о том, сколько секунд осталось до обновления квоты, и `RateLimit-ResetTime` header с такой же информацией, но в формате даты и времени. Все ответы, ограниченные или нет, содержат в себе `RateLimit-Limit`, `RateLimit-Remaining`, и некоторые другие headers, которые можно использовать для отслеживания использования лимитов до того, как достигнется лимит. 
### Пагинация
GitLab поддерживает два вида пагинации:
- Пагинация смещением. Метод по дефолту доступный на всех endpoints, кроме `users` endpoint.
- Keyset-based Пагинация. Была добавлена в несколько endpoints но была rolled out.
>[!important]
>Для больших коллекций лучше использовать keyset пагинацию, вместо пагинацию смещение, из-за производительности первого.

Иногда, возвращаемый результат может быть разделен на несколько страниц. Когда мы выводим список ресурсов, можно указать следующие параметры:
- `page` - номер страницы (default: 1)
- `per_page` - сколько результатов будет на одной страницу (default: 20, max: 100)
Также, как и в GitHub при пагинации в ответе на запрос возвращается `link` header, который содержит в себе URL и `rel` установленный в состояние `prev`, `next`, `first` или `last`.

Пример запроса:
```shell
curl --request GET \
  --head \
  --header "PRIVATE-TOKEN: <your_access_token>" \
  --url "https://gitlab.example.com/api/v4/projects/9/issues/8/notes?per_page=3&page=2"
```
 Пример ответа:
 ```http
 HTTP/2 200 OK
cache-control: no-cache
content-length: 1103
content-type: application/json
date: Mon, 18 Jan 2016 09:43:18 GMT
link: <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; rel="prev", <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=3&per_page=3>; rel="next", <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=1&per_page=3>; rel="first", <https://gitlab.example.com/api/v4/projects/8/issues/8/notes?page=3&per_page=3>; rel="last"
status: 200 OK
vary: Origin
x-next-page: 3
x-page: 2
x-per-page: 3
x-prev-page: 1
x-request-id: 732ad4ee-9870-4866-a199-a9db0cde3c86
x-runtime: 0.108688
x-total: 8
x-total-pages: 3
 ```


| Header          | Description                              |
| --------------- | ---------------------------------------- |
| `x-next-page`   | Индекс следующий страницы                |
| `x-page`        | Индекс текущей страницы (начинается с 1) |
| `x-per-page`    | Количество элементов на страницу         |
| `x-prev-page`   | Индекс предыдущей страницы               |
| `x-total`       | Все количество элементов                 |
| `x-total-pages` | Все количество страниц                   |
|                 |                                          |
|                 |                                          |
Keyset-based Пагинация позволяет более эффективно извлекать страницы и в сравнение с offset-based пагинацией - runtime не зависит от размера коллекции.
Этот метод контролируется следующими параметрами. `order_by` и `sort` обязательны.

| Параметр     | Обязателен | Объяснение                                                       |
| ------------ | ---------- | ---------------------------------------------------------------- |
| `pagination` | Да         | `keyset` (для того, чтобы разблокировать keyset-based пагинацию) |
| `per_page`   | Нет        | Количество элементов на страницу                                 |
| `order_by`   | Да         | Колонка, по которой будет упорядочивание                         |
| ``sort       | Да         | Направление сортировки (desc или asc)                            |


# 13. Нормализованная модель платформенных данных для RepoPulse

## Принцип нормализации

GitHub и GitLab отдают структурно разные ответы на одни и те же по смыслу сущности (`Pull Request` vs `Merge Request`, `Review` vs `Approvals + Discussions`, `Workflow Run` vs `Pipeline` и т.д.). Чтобы LLM получала на вход один предсказуемый JSON независимо от платформы, нужен слой адаптеров:

```text
GitHub REST API  ──▶ GitHubAdapter ──┐
                                      ├──▶ Normalized Model (Go structs) 
GitLab REST API  ──▶ GitLabAdapter ──┘
```

Каждый адаптер реализует общий интерфейс сбора данных, а дальше приложение работает только с нормализованной моделью и не знает, откуда пришли данные:

```go
type RepoIdentifier struct {
	Platform Platform
	Owner    string // GitHub owner / GitLab namespace
	Name     string // repo / project name
}

type Collector interface {
	FetchRepository(ctx context.Context, id RepoIdentifier) (Repository, error)
	FetchBranches(ctx context.Context, id RepoIdentifier) ([]Branch, error)
	FetchCommits(ctx context.Context, id RepoIdentifier, since time.Time) ([]Commit, error)
	FetchPullRequests(ctx context.Context, id RepoIdentifier) ([]PullRequest, error)
	FetchIssues(ctx context.Context, id RepoIdentifier) ([]Issue, error)
	FetchPipelines(ctx context.Context, id RepoIdentifier) ([]Pipeline, error)
}
```

`GitHubCollector` и `GitLabCollector` реализуют этот интерфейс, каждый — своим набором REST-запросов, описанных выше в документе, но возвращают одинаковые Go-структуры.

Два общих правила нормализации, которые применяются почти везде ниже:

1. **Коарс-enum + raw-поле.** Там, где у платформ разные наборы статусов (`state` у MR, `status`/`conclusion` у workflow run, `status` у pipeline), в модели есть узкий общий enum для сравнимости между платформами, и рядом — `Raw*` строковое поле с оригинальным значением, чтобы не терять нюанс, который может быть полезен LLM.
2. **Actor вместо User/Author.** GitHub `user`/`author`/`assignee` и GitLab `author`/`assignee`/`approved_by[].user` сводятся к одной структуре `Actor`.

## Go-структуры

```go
package model

import "time"

// Platform — исходная платформа репозитория.
type Platform string

const (
	PlatformGitHub Platform = "github"
	PlatformGitLab Platform = "gitlab"
)

// Actor — платформенный пользователь (автор PR/issue/коммента, ревьюер, assignee).
type Actor struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Name       string `json:"name,omitempty"`
	ProfileURL string `json:"profile_url,omitempty"`

	// AuthorAssociation — отношение автора к репозиторию (OWNER, MEMBER,
	// CONTRIBUTOR, NONE...). Нативно есть только у GitHub (author_association);
	// для GitLab оставляем пустым 
	AuthorAssociation string `json:"author_association,omitempty"`
}

// GitIdentity — git-автор/committer коммита (имя+email из самого коммита,
// а не платформенный аккаунт — они могут не совпадать).
type GitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
```

### Repository

```go
type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPrivate  Visibility = "private"
	VisibilityInternal Visibility = "internal" // только GitLab
)

type License struct {
	Key  string `json:"key,omitempty"`  // SPDX id (GitHub license.spdx_id) / GitLab license.key
	Name string `json:"name,omitempty"`
}

type RepositoryFeatures struct {
	IssuesEnabled        bool `json:"issues_enabled"`
	WikiEnabled          bool `json:"wiki_enabled"`
	MergeRequestsEnabled bool `json:"merge_requests_enabled"` // на GitHub Pull Requests всегда доступны
}

type Repository struct {
	Platform Platform `json:"platform"`
	// NativeID — исходный ID ресурса на платформе (GitHub repo id / GitLab project id).
	NativeID    string   `json:"native_id"`
	Owner       string   `json:"owner"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"` // full_name / name_with_namespace
	Description string   `json:"description,omitempty"`
	Topics      []string `json:"topics,omitempty"`

	// PrimaryLanguage доступен напрямую только у GitHub (language).
	// GitLab язык напрямую не отдаёт — поле можно оставить пустым либо
	// заполнять эвристикой по расширениям файлов, если приложение это делает.
	PrimaryLanguage string `json:"primary_language,omitempty"`

	Visibility Visibility `json:"visibility"`
	IsFork     bool       `json:"is_fork"`
	IsArchived bool       `json:"is_archived"`
	IsEmpty    bool       `json:"is_empty,omitempty"` // GitLab: empty_repo

	DefaultBranch string `json:"default_branch"`

	StarsCount    int `json:"stars_count"` // GitHub stargazers_count / GitLab star_count
	ForksCount    int `json:"forks_count"`
	WatchersCount int `json:"watchers_count,omitempty"` // только GitHub

	OpenIssuesCount int `json:"open_issues_count"`

	License *License `json:"license,omitempty"`

	Features RepositoryFeatures `json:"features"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// LastActivityAt — ключевой сигнал "живости" репозитория:
	// GitHub pushed_at / GitLab last_activity_at.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`

	WebURL string `json:"web_url"`
}
```

### Branch и Commit

```go
type Branch struct {
	Name          string    `json:"name"`
	IsDefault     bool      `json:"is_default"`
	IsProtected   bool      `json:"is_protected"`
	LastCommitSHA string    `json:"last_commit_sha"`
	LastCommitAt  time.Time `json:"last_commit_at,omitempty"`

	// IsMerged — напрямую отдаёт только GitLab (merged). Для GitHub
	// вычисляется приложением (сравнением с default branch), если нужно.
	IsMerged *bool `json:"is_merged,omitempty"`
}

type CommitStats struct {
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
	Total     int `json:"total"`
}

type Commit struct {
	SHA      string `json:"sha"`
	ShortSHA string `json:"short_sha,omitempty"`
	Message  string `json:"message"`

	Author      GitIdentity `json:"author"`
	AuthoredAt  time.Time   `json:"authored_at"`
	Committer   GitIdentity `json:"committer"`
	CommittedAt time.Time   `json:"committed_at"`

	// AuthorActor — связанный платформенный аккаунт автора, если GitHub/GitLab
	// смогли сопоставить email коммита с зарегистрированным пользователем.
	// Может быть nil (коммит от внешнего/неаккаунтного email).
	AuthorActor *Actor `json:"author_actor,omitempty"`

	ParentSHAs    []string `json:"parent_shas,omitempty"`
	IsMergeCommit bool     `json:"is_merge_commit"` // len(ParentSHAs) > 1

	// Stats: GitHub отдаёт всегда в /commits/{sha}, GitLab — только при with_stats=true.
	Stats *CommitStats `json:"stats,omitempty"`

	WebURL string `json:"web_url,omitempty"`
}
```

### Pull Request / Merge Request

```go
type PullRequestState string

const (
	PRStateOpen   PullRequestState = "open"
	PRStateClosed PullRequestState = "closed" // закрыт без слияния
	PRStateMerged PullRequestState = "merged"
)

type FileStatus string

const (
	FileAdded    FileStatus = "added"
	FileModified FileStatus = "modified"
	FileRemoved  FileStatus = "removed"
	FileRenamed  FileStatus = "renamed"
)

type FileChange struct {
	Path    string     `json:"path"`
	OldPath string     `json:"old_path,omitempty"` // заполняется при renamed
	Status  FileStatus `json:"status"`

	// Additions/Deletions GitHub отдаёт напрямую (files[].additions/deletions).
	// GitLab в /diffs отдаёт только raw diff — счётчики нужно получать
	// парсингом diff-текста на стороне приложения.
	Additions int `json:"additions,omitempty"`
	Deletions int `json:"deletions,omitempty"`
}

type ChangeStats struct {
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedFiles int `json:"changed_files"`
}

type CommentKind string

const (
	CommentKindDiscussion CommentKind = "discussion" // обычный комментарий в общей ленте
	CommentKindSystem     CommentKind = "system"      // авто-запись (GitLab system note: смена статуса, ассайн и т.п.)
)

// Comment — комментарий верхнего уровня (GitHub issue comment / GitLab note,
// не привязанный к конкретной строке diff).
type Comment struct {
	ID        string      `json:"id"`
	Kind      CommentKind `json:"kind"`
	Body      string      `json:"body"`
	Author    Actor       `json:"author"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at,omitempty"`
}

type DiffSide string

const (
	DiffSideOld DiffSide = "old"
	DiffSideNew DiffSide = "new"
)

// ReviewComment — комментарий, привязанный к конкретной строке diff
// (GitHub review comment / GitLab DiffNote).
type ReviewComment struct {
	ID       string   `json:"id"`
	Body     string   `json:"body"`
	Author   Actor    `json:"author"`
	FilePath string   `json:"file_path"`
	Line     *int     `json:"line,omitempty"`
	Side     DiffSide `json:"side,omitempty"`

	CommitSHA   string `json:"commit_sha,omitempty"`
	InReplyToID string `json:"in_reply_to_id,omitempty"`

	// IsResolved: у GitLab это нативное поле note.resolved. У GitHub
	// такого поля нет вообще — resolve относится к целому review thread
	// через GraphQL, а не REST; при использовании только REST оставляем nil.
	IsResolved *bool `json:"is_resolved,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

type ReviewState string

const (
	ReviewApproved         ReviewState = "approved"
	ReviewChangesRequested ReviewState = "changes_requested"
	ReviewCommented        ReviewState = "commented"
	ReviewDismissed        ReviewState = "dismissed"
	ReviewPending          ReviewState = "pending"
)

// Review — единичный акт ревью. Для GitHub маппится 1:1 из Pull Request Review.
// У GitLab нативной сущности "review" нет — Review синтезируется из
// Merge Request Approvals API (каждый approved_by[] → Review{State: approved}).
type Review struct {
	ID          string      `json:"id"`
	Reviewer    Actor       `json:"reviewer"`
	State       ReviewState `json:"state"`
	Body        string      `json:"body,omitempty"`
	SubmittedAt time.Time   `json:"submitted_at"`
	CommitSHA   string      `json:"commit_sha,omitempty"`
}

type PullRequest struct {
	NativeID string `json:"native_id"`
	Number   int    `json:"number"` // GitHub number / GitLab iid

	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	State       PullRequestState `json:"state"`
	// RawState сохраняет исходное значение (например, GitLab "locked"),
	// которое коарс-enum State огрубляет до "open".
	RawState string `json:"raw_state,omitempty"`
	IsDraft  bool   `json:"is_draft"`

	Author    Actor   `json:"author"`
	Assignees []Actor `json:"assignees,omitempty"`
	// RequestedReviewers — назначенные, но ещё не оставившие Review.
	RequestedReviewers []Actor `json:"requested_reviewers,omitempty"`

	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	HeadSHA      string `json:"head_sha,omitempty"`

	Labels []string `json:"labels,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	MergedBy  *Actor     `json:"merged_by,omitempty"`

	ChangeStats *ChangeStats `json:"change_stats,omitempty"`
	Files       []FileChange `json:"files,omitempty"`

	Comments       []Comment       `json:"comments,omitempty"`
	Reviews        []Review        `json:"reviews,omitempty"`
	ReviewComments []ReviewComment `json:"review_comments,omitempty"`

	CommitsCount int `json:"commits_count,omitempty"`

	WebURL string `json:"web_url"`
}
```

### Issue

```go
type IssueState string

const (
	IssueStateOpen   IssueState = "open"
	IssueStateClosed IssueState = "closed"
)

type Issue struct {
	NativeID string     `json:"native_id"`
	Number   int        `json:"number"`
	Title    string     `json:"title"`
	Body     string     `json:"body,omitempty"`
	State    IssueState `json:"state"`

	Author    Actor    `json:"author"`
	Assignees []Actor  `json:"assignees,omitempty"`
	Labels    []string `json:"labels,omitempty"`

	CommentsCount int `json:"comments_count"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	ClosedBy  *Actor     `json:"closed_by,omitempty"`

	WebURL string `json:"web_url"`
}
```

> [!important]
> GitHub `List repository issues` возвращает вперемешку и issues, и pull requests (PR — тоже issue). При сборе данных такие записи (наличие поля `pull_request` в ответе) нужно отфильтровывать ещё в адаптере и не пускать в `[]Issue` — иначе PR задвоятся между `Issues` и `PullRequests` в снэпшоте.

### CI/CD: Pipeline, Job, CommitStatus

```go
// PipelineStatus — общий коарс-статус для GitHub Workflow Run (status+conclusion)
// и GitLab Pipeline (status).
type PipelineStatus string

const (
	PipelineStatusPending  PipelineStatus = "pending"
	PipelineStatusRunning  PipelineStatus = "running"
	PipelineStatusSuccess  PipelineStatus = "success"
	PipelineStatusFailed   PipelineStatus = "failed"
	PipelineStatusCanceled PipelineStatus = "canceled"
	PipelineStatusSkipped  PipelineStatus = "skipped"
)

type Job struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Stage  string         `json:"stage,omitempty"` // нативно только у GitLab
	Status PipelineStatus `json:"status"`
	// RawStatus/RawConclusion — исходные значения платформы
	// (GitHub: status="in_progress"+conclusion="failure"; GitLab: status="failed").
	RawStatus string `json:"raw_status,omitempty"`

	AllowFailure bool `json:"allow_failure,omitempty"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DurationSec *int       `json:"duration_seconds,omitempty"`

	WebURL string `json:"web_url,omitempty"`
}

// Pipeline — нормализованный GitHub Workflow Run / GitLab Pipeline.
type Pipeline struct {
	NativeID string `json:"native_id"`
	// Name: GitHub workflow name / GitLab pipeline name (может быть пустым).
	Name string `json:"name,omitempty"`

	Ref       string         `json:"ref"`
	CommitSHA string         `json:"commit_sha"`
	Status    PipelineStatus `json:"status"`
	RawStatus string         `json:"raw_status,omitempty"`

	// TriggerEvent: GitHub event (push, pull_request, schedule...) /
	// GitLab source (push, merge_request_event, schedule...).
	TriggerEvent string `json:"trigger_event,omitempty"`

	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
	DurationSec *int       `json:"duration_seconds,omitempty"`

	Jobs []Job `json:"jobs,omitempty"`

	WebURL string `json:"web_url,omitempty"`
}

// CommitStatus — Commit Status API GitHub/GitLab (отдельная от Pipeline
// сущность: внешние системы могут выставлять статус commit'у сами по себе).
type CommitStatus struct {
	Context     string    `json:"context"` // GitHub context / GitLab name
	State       string    `json:"state"`   // error | failure | pending | success
	Description string    `json:"description,omitempty"`
	TargetURL   string    `json:"target_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
```

### Верхнеуровневый снэпшот для LLM

```go
// RepoHealthSnapshot — единый payload, который целиком сериализуется в JSON
// и отправляется в LLM для оценки здоровья репозитория.
type RepoHealthSnapshot struct {
	CollectedAt time.Time  `json:"collected_at"`
	Repository  Repository `json:"repository"`

	Branches     []Branch      `json:"branches"`
	Commits      []Commit      `json:"commits"`
	PullRequests []PullRequest `json:"pull_requests"`
	Issues       []Issue       `json:"issues"`
	Pipelines    []Pipeline    `json:"pipelines"`
}
```

## Что не нормализуется 1:1 и требует решения на уровне приложения

| Сущность / поле | GitHub | GitLab | Как поступаем |
|---|---|---|---|
| Diff-статистика файла | `additions`/`deletions` в ответе | Только raw `diff`, нужен парсинг | Парсим diff в адаптере GitLab перед заполнением `FileChange` |
| Резолв ревью-комментария | Нет в REST (есть только в GraphQL) | `notes[].resolved` нативно | `ReviewComment.IsResolved` — `nil` для GitHub |
| "Review" как сущность | Нативная `Pull Request Review` | Отсутствует, собирается из Approvals API | `Review` для GitLab синтезируется адаптером, `Reviewer.AuthorAssociation` всегда пуст |
| Язык репозитория | `language` в ответе | Не отдаётся REST API репозитория | `PrimaryLanguage` пуст для GitLab, если не считать отдельно |
| `author_association` | Есть у комментариев/PR/issues | Аналога нет | Поле остаётся пустым для GitLab-объектов |
| Статусы pipeline/job | `status` + `conclusion` (два поля) | Один `status` с более широким набором значений (`manual`, `scheduled` и т.д.) | Огрубляем оба в `PipelineStatus`, оригинал кладём в `RawStatus` |

## Рекомендации по сборке payload для LLM

- Для активных репозиториев `Commits`/`PullRequests`/`Issues` могут исчисляться тысячами — весь список в LLM целиком отправлять не стоит (контекст и стоимость). Практичный вариант: собирать `RepoHealthSnapshot` с окном по времени (например, последние 90 дней активности) плюс отдельно precomputed-агрегаты (частота коммитов, среднее time-to-merge, доля PR с ревью, pass rate последних N pipeline) как компактную сводку, а сырые списки передавать в урезанном/сэмплированном виде или вообще не передавать, если агрегатов достаточно для оценки.
- Поля вроде `Body`/`Description`/`Message` стоит обрезать по длине перед сериализацией — это единственные поля с произвольно большим объёмом текста в модели.

## Состояние сбора коллекций

Пустой массив объектов сам по себе не позволяет определить, действительно ли данные отсутствуют на платформе. Например, `comments: []` может означать как отсутствие комментариев, так и то, что запрос не был успешно выполнен, доступ к данным запрещен, был исчерпан rate limit или сбор завершился до получения всех страниц.

Поэтому вместе с каждой собираемой коллекцией RepoPulse должен хранить метаданные о состоянии ее получения.

Для коллекции необходимо сохранять:

| Поле | Описание |
|---|---|
| `collection_status` | Состояние выполнения сбора коллекции. Например: `success`, `partial`, `failed`, `rate_limited`, `forbidden`, `unauthorized`. |
| `is_complete` | Признак того, что RepoPulse успешно обработал все доступные страницы и коллекция получена полностью. |
| `window_from` | Начало временного интервала, за который выполнялся сбор данных. |
| `window_to` | Конец временного интервала, за который выполнялся сбор данных. |
| `pages_fetched` | Количество успешно обработанных страниц API. |
| `items_fetched` | Количество полученных объектов. |
| `errors` | Ошибки, возникшие во время сбора данных. |
| `collected_at` | Время завершения или последней попытки сбора коллекции. |

Ошибка должна содержать информацию об источнике, на котором произошел сбой:

```text
CollectionError
├── provider
├── resource
├── endpoint
├── page
├── http_status
├── error_code
└── message
```

Например:

```json
{
  "items": [],
  "collection_status": "success",
  "is_complete": true,
  "window_from": "2026-09-01T00:00:00Z",
  "window_to": "2026-09-26T00:00:00Z",
  "pages_fetched": 1,
  "items_fetched": 0,
  "errors": []
}
```

В данном случае пустой `items` однозначно означает, что за указанный временной интервал объекты действительно отсутствуют.

Если же сбор завершился частично:

```json
{
  "items": [...],
  "collection_status": "partial",
  "is_complete": false,
  "pages_fetched": 4,
  "items_fetched": 400,
  "errors": [
    {
      "provider": "github",
      "resource": "pull_request_comments",
      "endpoint": "/repos/{owner}/{repo}/issues/{issue_number}/comments",
      "page": 5,
      "http_status": 403,
      "error_code": "rate_limit_exceeded",
      "message": "API rate limit exceeded"
    }
  ]
}
```

Таким образом, состояние данных необходимо отделять от самих данных:

```text
items = [] + is_complete = true
    → данных действительно нет

items = [] + is_complete = false
    → отсутствие данных не подтверждено

items != [] + is_complete = false
    → получена только часть данных
```

Это позволяет RepoPulse отличать фактическое отсутствие активности в репозитории от проблем интеграции и локализовать источник неполных данных.

# 14. Технический прототип запросов

Ниже — минимальный набор запросов, которые вместе вытаскивают из публичного репозитория всё, что описано в нормализованной модели (раздел 13). Для публичных репозиториев токен не обязателен, но снижает шанс упереться в анонимный rate limit.

В качестве тестового репозитория для GitHub используется `microsoft/vscode`, для GitLab — `gitlab-org/gitlab-runner` (оба публичные, активные, с PR/MR, issues и CI/CD). `OWNER/REPO` и `:id` проекта можно заменить на любой другой публичный репозиторий — остальная часть запросов не меняется.

Значения вроде `PULL_NUMBER`, `MERGE_REQUEST_IID`, `RUN_ID`, `PIPELINE_ID`, `SHA` — заведомо динамические: их нужно подставлять из ответа предыдущего запроса (например, `PULL_NUMBER` — это `number` из списка pull request).

## GitHub

### Запрос репозитория

Базовые метаданные: описание, звёзды, форки, лицензия, default branch, архивирован/форк ли.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/microsoft/vscode
```

### Список веток

Для сигналов защищённости веток и того, какая ветка дефолтная/актуальная.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/branches?per_page=100"
```

### Список коммитов

История активности: частота коммитов, авторы, даты — основа для оценки "живости" репозитория.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/commits?per_page=100"
```

### Список pull request

Все PR (открытые и закрытые) — основа для оценки процесса код-ревью и скорости слияния.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/pulls?state=all&per_page=100"
```

### Файлы конкретного pull request

Что именно менялось в PR — размер и характер изменений.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/pulls/PULL_NUMBER/files"
```

### Обычные комментарии pull request

Комментарии в общей ленте PR (PR — это тоже issue, поэтому используется issues-endpoint).

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/issues/PULL_NUMBER/comments"
```

### Review-комментарии pull request (к строкам diff)

Комментарии, оставленные непосредственно на изменённых строках кода во время код-ревью.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/pulls/PULL_NUMBER/comments"
```

### Reviews pull request

Итоговые решения ревьюеров: `APPROVED`, `CHANGES_REQUESTED`, `COMMENTED` — ключевой сигнал качества процесса ревью.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/pulls/PULL_NUMBER/reviews"
```

### Список issues

Открытые/закрытые issues репозитория — сигнал реагирования мейнтейнеров на баги и запросы.

> [!important]
> Ответ содержит вперемешку issues и pull requests — записи с полем `pull_request` нужно отфильтровать.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/issues?state=all&per_page=100"
```

### Workflows репозитория

Список настроенных CI/CD workflows (GitHub Actions).

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  https://api.github.com/repos/microsoft/vscode/actions/workflows
```

### Запуски workflow репозитория

История запусков CI: статусы, продолжительность, частота падений.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/actions/runs?per_page=100"
```

### Jobs конкретного workflow run

Детализация запуска по jobs/steps.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/actions/runs/RUN_ID/jobs"
```

### Commit status конкретного коммита/ветки

Объединённый статус внешних CI-систем для commit/branch/tag.

```shell
curl -L \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2026-03-10" \
  "https://api.github.com/repos/microsoft/vscode/commits/SHA/status"
```

## GitLab

### Запрос репозитория (проекта)

Базовые метаданные проекта: описание, звёзды, форки, лицензия, default branch, видимость.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner"
```

### Список веток

Для сигналов защищённости веток и актуальности default branch.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/repository/branches?per_page=100"
```

### Список коммитов

История активности проекта: частота коммитов, авторы, даты.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/repository/commits?per_page=100"
```

### Список merge request

Все MR (открытые, закрытые, слитые) — основа для оценки процесса код-ревью и скорости слияния.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/merge_requests?state=all&per_page=100"
```

### Diffs файлов merge request

Что именно менялось в MR — размер и характер изменений (raw diff, без готовых счётчиков additions/deletions).

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/merge_requests/MERGE_REQUEST_IID/diffs?per_page=100"
```

### Notes (комментарии) merge request

Комментарии и системные записи в общей ленте MR.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/merge_requests/MERGE_REQUEST_IID/notes?per_page=100"
```

### Discussions merge request

Структурированные обсуждения с сохранением цепочек ответов и привязкой к diff (`DiffNote`) — нужны для восстановления комментариев к конкретным строкам кода.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/merge_requests/MERGE_REQUEST_IID/discussions?per_page=100"
```

### Approvals merge request

Кто одобрил MR и удовлетворяет ли он approval-требованиям — аналог GitHub review state.

> [!important]
> Требует аутентификации даже для публичного проекта — нужен `PRIVATE-TOKEN`.

```shell
curl --header "PRIVATE-TOKEN: <your_access_token>" \
  --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/merge_requests/MERGE_REQUEST_IID/approvals"
```

### Список issues проекта

Открытые/закрытые issues репозитория — сигнал реагирования мейнтейнеров на баги и запросы.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/issues?state=all&per_page=100"
```

### Pipelines проекта

История запусков CI/CD: статусы, частота падений.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/pipelines?per_page=100"
```

### Конкретный pipeline

Детали конкретного запуска: длительность, время старта/финиша, coverage.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/pipelines/PIPELINE_ID"
```

### Jobs конкретного pipeline

Детализация pipeline по jobs/stages.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/pipelines/PIPELINE_ID/jobs?per_page=100"
```

### Commit statuses конкретного коммита

Статусы внешних CI-систем, привязанные к конкретному commit SHA.

```shell
curl --header "Accept: application/json" \
  --url "https://gitlab.com/api/v4/projects/gitlab-org%2Fgitlab-runner/repository/commits/SHA/statuses"
```


# 15. Выбор платформы для первой версии MVP

Для первой версии RepoPulse предлагается реализовать интеграцию с **GitHub**, а поддержку GitLab добавить следующим этапом.

GitHub подходит для первой реализации по следующим причинам:

- исследуемые сущности репозитория доступны через REST API и позволяют получить необходимую для RepoPulse информацию о репозитории, commits, Pull Requests, Issues, comments, reviews и CI/CD;
- GitHub Actions предоставляет иерархию `Workflow → Workflow Run → Job → Step`, позволяющую получить детальную информацию о выполнении CI/CD;
- на одной платформе можно реализовать и проверить полный цикл сбора, нормализации и анализа данных до добавления второго provider;
- GitHub удобно использовать для проверки интеграции на публичных репозиториях без необходимости предварительно создавать собственные проекты с историей активности.

При этом внутренняя модель RepoPulse не должна зависеть от GitHub. На уровне приложения необходимо использовать нормализованные сущности:

```text
Repository
ChangeRequest
Issue
Commit
Comment
Review
PipelineRun
PipelineJob
...
```

GitHub и GitLab должны выступать отдельными источниками данных:

```text
GitHub API ─┐
            ├──> Provider adapters ──> Normalized model ──> RepoPulse
GitLab API ─┘
```

Такой подход позволяет сначала реализовать `GitHubProvider`, не закладывая GitHub-специфичные структуры в бизнес-логику RepoPulse, а затем добавить `GitLabProvider`, преобразующий Merge Request, Pipeline, Job и другие сущности GitLab в ту же нормализованную модель.

Следовательно, порядок реализации для MVP:

```text
1. GitHub API
2. Нормализация полученных данных
3. Расчет метрик здоровья репозитория
4. GitLab API
5. Преобразование GitLab-сущностей в существующую нормализованную модель
```
