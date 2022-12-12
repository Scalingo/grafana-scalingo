load('scripts/drone/vault.star', 'from_secret', 'pull_secret')
load('scripts/drone/steps/lib.star', 'publish_image', 'compile_build_cmd')

aquasec_trivy_image = 'aquasec/trivy:0.21.0'

def cronjobs(edition):
    grafana_com_nightly_pipeline = cron_job_pipeline(
        cronName='grafana-com-nightly',
        name='grafana-com-nightly',
        steps=[compile_build_cmd(),post_to_grafana_com_step()]
    )
    return [
        scan_docker_image_pipeline(edition, 'latest'),
        scan_docker_image_pipeline(edition, 'main'),
        scan_docker_image_pipeline(edition, 'latest-ubuntu'),
        scan_docker_image_pipeline(edition, 'main-ubuntu'),
        grafana_com_nightly_pipeline,
    ]

def cron_job_pipeline(cronName, name, steps):
    return {
        'kind': 'pipeline',
        'type': 'docker',
        'platform': {
            'os': 'linux',
            'arch': 'amd64',
        },
        'name': name,
        'trigger': {
            'event': 'cron',
            'cron': cronName,
        },
        'clone': {
            'retries': 3,
        },
        'steps': steps,
    }

def scan_docker_image_pipeline(edition, tag):
    if edition != 'oss':
        edition='grafana-enterprise'
    else:
        edition='grafana'

    dockerImage='grafana/{}:{}'.format(edition, tag)

    return cron_job_pipeline(
        cronName='nightly',
        name='scan-' + dockerImage + '-image',
        steps=[
            scan_docker_image_unkown_low_medium_vulnerabilities_step(dockerImage),
            scan_docker_image_high_critical_vulnerabilities_step(dockerImage),
            slack_job_failed_step('grafana-backend-ops', dockerImage),
        ])

def scan_docker_image_unkown_low_medium_vulnerabilities_step(dockerImage):
    return {
        'name': 'scan-unkown-low-medium-vulnerabilities',
        'image': aquasec_trivy_image,
        'commands': [
            'trivy --exit-code 0 --severity UNKNOWN,LOW,MEDIUM ' + dockerImage,
        ],
    }

def scan_docker_image_high_critical_vulnerabilities_step(dockerImage):
    return {
        'name': 'scan-high-critical-vulnerabilities',
        'image': aquasec_trivy_image,
        'commands': [
            'trivy --exit-code 1 --severity HIGH,CRITICAL ' + dockerImage,
        ],
    }

def slack_job_failed_step(channel, image):
    return {
        'name': 'slack-notify-failure',
        'image': 'plugins/slack',
        'settings': {
            'webhook': from_secret('slack_webhook_backend'),
            'channel': channel,
            'template': 'Nightly docker image scan job for ' + image + ' failed: {{build.link}}',
        },
        'when': {
            'status': 'failure'
        }
    }

def post_to_grafana_com_step():
    return {
            'name': 'post-to-grafana-com',
            'image': publish_image,
            'environment': {
                'GRAFANA_COM_API_KEY': from_secret('grafana_api_key'),
                'GCP_KEY': from_secret('gcp_key'),
            },
            'depends_on': ['compile-build-cmd'],
            'commands': ['./bin/build publish grafana-com --edition oss'],
        }

