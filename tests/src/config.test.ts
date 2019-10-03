import * as types from '@claw/types';
import * as mock from 'mock-fs';
import * as fs from 'fs';
import * as YAML from 'js-yaml';

test('loadConfigFile returns default yml file path', () => {
    const app = require('@claw/app').default;
    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    const [config, configFile] = app.loadConfigFile(null);

    expect(config).toMatchObject({});
    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('loadConfigFile from custom yml', () => {
    const app = require('@claw/app').default;

    mock({
        'cla-worker-foo.yml': '{ url: "foobar" }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    const [config, configFile] = app.loadConfigFile('cla-worker-foo.yml');

    mock.restore();

    expect(config).toMatchObject({ url: 'foobar' });
    expect(configFile).toMatch(/cla-worker-foo.yml$/);
});

test('saveConfigFile to custom yml', () => {
    const app = require('@claw/app').default;

    mock({
        'cla-worker.yml': '{ url: "foobarbar" }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            config: 'cla-worker.yml'
        }
    });

    const [configFile, dump] = app.saveConfigFile({ foobar: 123 });

    const yaml = fs.readFileSync('cla-worker.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        url: 'foobarbar',
        foobar: 123,
        registrations: []
    });

    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('saveConfigFile registration to default yml', () => {
    const app = require('@claw/app').default;

    mock({});

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            save: true
        }
    });

    const [configFile, dump] = app.saveConfigFile({
        registrations: [{ id: 'foo', token: 'bar' }]
    });

    const yaml = fs.readFileSync('cla-worker.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        registrations: [{ id: 'foo', token: 'bar' }]
    });

    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('saveConfigFile registration to existing yml with registrations', () => {
    const app = require('@claw/app').default;

    mock({
        'cla-worker.yml':
            '{ registrations: [{id: "firstid", token: "firsttoken"}] }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            save: true
        }
    });

    const [configFile, dump] = app.saveConfigFile({
        registrations: [{ id: 'foo', token: 'bar' }]
    });

    const yaml = fs.readFileSync('cla-worker.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        registrations: [
            { id: 'firstid', token: 'firsttoken' },
            { id: 'foo', token: 'bar' }
        ]
    });

    expect(configFile).toMatch(/cla-worker.yml$/);
});
