# Сбор прямых зависимостей

Сборщик internal/analyzer/application/dependencies.Collector работает с закреплённым
Git-снимком через worker.GitSession. Штатный путь: app.Run ->
activity.Processor.Process -> Collector.Collect -> snapshot.Build.
Сбор не выполняет команды анализируемого проекта и не обращается в OSV.

## Корневые файлы и версии

Один проход по GitData.Files выбирает только точные корневые пути. Обычные файлы
читаются по ObjectSHA однократно; символьные ссылки не интерпретируются как
манифесты. Наличие go.work устанавливается по дереву без чтения содержимого.
go.sum, вложенные манифесты и транзитивные записи не используются.

| Профиль | Прямые записи | manifest_path | version_source |
|---|---|---|---|
| npm | Объединение dependencies/devDependencies/optionalDependencies/peerDependencies; версия из packages[node_modules/name] корневого lock v2/v3 | package.json | npm_lock_v2 / npm_lock_v3 |
| Maven | project/dependencies, включая test/provided/runtime/system и optional; локальные properties и dependencyManagement сопоставляются по groupId/artifactId/type/classifier | pom.xml | maven_declaration / maven_dependency_management |
| Go | Одиночные/блочные require без indirect, разобранные x/mod/modfile | go.mod | go_require_declaration |

npm-ссылки и aliases не подменяются пакетами с предполагаемой версией. Maven
dependencyManagement и импортируемые BOM не добавляют прямые зависимости.
Неразрешённые свойства, диапазоны, SNAPSHOT, внешний parent/BOM и profiles
фиксируются как ограничения; пригодные локальные декларации сохраняются.
В Go сохраняются версии деклараций, включая prerelease и псевдоверсии;
влияющие на прямые записи replace/exclude и наличие go.work означают неполноту.
Это не подтверждает фактический состав сборки Maven/Go.

## Полнота и ограничения

В Collection.Sources используются источник git и scopes dependencies_npm,
dependencies_maven, dependencies_go, а также суммарный dependencies.
Без подтверждённого полного дерева отсутствие манифеста не считается доказанным.
Отсутствующий профиль при полном дереве имеет not_applicable;
разобранный пустой профиль — complete с нулевой выборкой.
Ошибки чтения, формата или разрешения версии дают partial, сохраняя пригодные записи
других файлов и объявлений. При отмене контекста сбор завершается с ошибкой отмены.

Причины содержат стабильные коды, безопасные тексты и cannot_conclude;
исходные сообщения библиотек и содержимое файлов не сохраняются.
Если у профиля несколько причин, первая по коду записывается в основной scope,
остальные — в отдельные scopes dependencies_<profile>/<CODE>.
Основные коды: MANIFEST_READ_FAILED, MANIFEST_UNSUPPORTED_MODE,
DEPENDENCY_TREE_INCOMPLETE, NPM_MANIFEST_INVALID, NPM_MANIFEST_MISSING,
NPM_LOCK_MISSING, NPM_LOCK_INVALID, NPM_LOCK_UNSUPPORTED,
NPM_REFERENCE_UNSUPPORTED, NPM_VERSION_UNRESOLVED,
MAVEN_MANIFEST_INVALID, MAVEN_DEPENDENCY_UNRESOLVED, MAVEN_VERSION_UNRESOLVED,
MAVEN_EXTERNAL_PARENT, MAVEN_EXTERNAL_BOM, MAVEN_PROFILES_UNSUPPORTED,
MAVEN_SCOPE_UNSUPPORTED, GO_MANIFEST_INVALID, GO_MANIFEST_MISSING,
GO_VERSION_UNRESOLVED, GO_REPLACE, GO_EXCLUDE, GO_WORKSPACE.

Общая нормализация снимка объединяет (ecosystem, package, version), сохраняет
все уникальные Origins и сортирует их по пути/источнику. Основные поля происхождения
берутся из первого отсортированного origin. После дедупликации применяется общий
лимит 20: ровно 20 допустимы, при 21 и более сохраняются первые 20 в порядке
ecosystem/package/version, суммарный scope получает partial / DEPENDENCY_LIMIT.
Полнота отдельных профилей описывает извлечение до этой общей выборки.
Поле total остаётся неизвестным при неполноте; число сохранённых уникальных записей
передаётся в collected суммарного scope.

Сборщик не вычисляет SEC_VULN_RISK и не объявляет отсутствие уязвимостей:

