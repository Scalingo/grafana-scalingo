load(
    'scripts/drone/steps/lib.star',
    'download_grabpl_step',
    'gen_version_step',
    'yarn_install_step',
    'wire_install_step',
    'identify_runner_step',
    'lint_drone_step',
    'lint_backend_step',
    'lint_frontend_step',
    'codespell_step',
    'shellcheck_step',
    'build_backend_step',
    'build_frontend_step',
    'build_frontend_package_step',
    'build_plugins_step',
    'test_backend_step',
    'test_backend_integration_step',
    'test_frontend_step',
    'package_step',
    'grafana_server_step',
    'e2e_tests_step',
    'e2e_tests_artifacts',
    'build_storybook_step',
    'copy_packages_for_docker_step',
    'build_docker_images_step',
    'postgres_integration_tests_step',
    'mysql_integration_tests_step',
    'redis_integration_tests_step',
    'memcached_integration_tests_step',
    'benchmark_ldap_step',
    'validate_scuemata_step',
    'ensure_cuetsified_step',
    'test_a11y_frontend_step',
)

load(
    'scripts/drone/services/services.star',
    'integration_test_services',
    'integration_test_services_volumes',
    'ldap_service',
)

load(
    'scripts/drone/utils/utils.star',
    'notify_pipeline',
    'pipeline',
    'failure_template',
    'drone_change_template',
)

load(
    'scripts/drone/pipelines/docs.star',
    'docs_pipelines',
    'trigger_docs',
)

ver_mode = 'pr'

def pr_pipelines(edition):
    services = integration_test_services(edition)
    volumes = integration_test_services_volumes()
    variants = ['linux-amd64', 'linux-amd64-musl', 'darwin-amd64', 'windows-amd64', 'armv6',]
    include_enterprise2 = edition == 'enterprise'
    init_steps = [
        identify_runner_step(),
        download_grabpl_step(),
        gen_version_step(ver_mode),
        wire_install_step(),
        yarn_install_step(),
    ]
    test_steps = [
        lint_drone_step(),
        codespell_step(),
        shellcheck_step(),
        lint_backend_step(edition=edition),
        lint_frontend_step(),
        test_backend_step(edition=edition),
        test_backend_integration_step(edition=edition),
        test_frontend_step(),
    ]
    build_steps = [
        build_backend_step(edition=edition, ver_mode=ver_mode, variants=variants),
        build_frontend_step(edition=edition, ver_mode=ver_mode),
        build_frontend_package_step(edition=edition, ver_mode=ver_mode),
        build_plugins_step(edition=edition),
        validate_scuemata_step(),
        ensure_cuetsified_step(),
    ]
    integration_test_steps = [
        postgres_integration_tests_step(edition=edition, ver_mode=ver_mode),
        mysql_integration_tests_step(edition=edition, ver_mode=ver_mode),
    ]

    if include_enterprise2:
        edition2 = 'enterprise2'
        build_steps.append(benchmark_ldap_step())
        services.append(ldap_service())
        test_steps.extend([
            lint_backend_step(edition=edition2),
            test_backend_step(edition=edition2),
            test_backend_integration_step(edition=edition2),
        ])
        build_steps.extend([
            build_backend_step(edition=edition2, ver_mode=ver_mode, variants=['linux-amd64']),
        ])

    # Insert remaining build_steps
    build_steps.extend([
        package_step(edition=edition, ver_mode=ver_mode, include_enterprise2=include_enterprise2, variants=variants),
        grafana_server_step(edition=edition),
        e2e_tests_step('dashboards-suite', edition=edition),
        e2e_tests_step('smoke-tests-suite', edition=edition),
        e2e_tests_step('panels-suite', edition=edition),
        e2e_tests_step('various-suite', edition=edition),
        e2e_tests_artifacts(edition=edition),
        build_storybook_step(edition=edition, ver_mode=ver_mode),
        test_a11y_frontend_step(ver_mode=ver_mode, edition=edition),
        copy_packages_for_docker_step(),
        build_docker_images_step(edition=edition, ver_mode=ver_mode, archs=['amd64',]),
    ])

    if include_enterprise2:
        integration_test_steps.extend([
            redis_integration_tests_step(edition=edition2, ver_mode=ver_mode),
            memcached_integration_tests_step(edition=edition, ver_mode=ver_mode),
        ])
        build_steps.extend([
            package_step(edition=edition2, ver_mode=ver_mode, include_enterprise2=include_enterprise2, variants=['linux-amd64']),
        ])

    trigger = {
        'event': [
            'pull_request',
        ],
        'paths': {
            'exclude': [
                'docs/**',
            ],
        },
    }

    return [
        pipeline(
            name='pr-test', edition=edition, trigger=trigger, services=[], steps=init_steps + test_steps,
        ), pipeline(
            name='pr-build-e2e', edition=edition, trigger=trigger, services=[], steps=init_steps + build_steps,
        ), pipeline(
            name='pr-integration-tests', edition=edition, trigger=trigger, services=services,
            steps=[download_grabpl_step(), identify_runner_step(),] + integration_test_steps,
            volumes=volumes,
        ), docs_pipelines(edition, ver_mode, trigger_docs())
    ]
