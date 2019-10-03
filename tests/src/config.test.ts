import * as mock from 'mock-fs';
import * as fs from 'fs';
import * as YAML from 'js-yaml';

afterEach(mock.restore);

test('load not existing config returns default yml file path', () => {
    const app = require('@claw/app').default;

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    const [config, configFile] = app.config.load();

    expect(config).toMatchObject({});
    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('load existing default config returns contents', () => {
    const app = require('@claw/app').default;
    const dir = process.cwd();

    mock({
        [`${dir}/cla-worker.yml`]: '{ url: "barfoo" }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    const [config, configFile] = app.config.load();

    mock.restore();

    expect(config).toMatchObject({ url: 'barfoo' });
    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('load config file called 0', () => {
    const app = require('@claw/app').default;

    mock({
        '0': '{ url: "barfoo" }'
    });

    app.build({
        argv: {}
    });

    const [config, configFile] = app.config.load('0');

    mock.restore();

    expect(config).toMatchObject({ url: 'barfoo' });
    expect(configFile).toMatch(/0$/);
});

test('app load fails on custom config that does not exist', () => {
    const app = require('@claw/app').default;

    try {
        app.build({
            argv: {
                _: ['testcmd'],
                $0: 'testcmd',
                config: 'mycustom-is-not-here.yml'
            }
        });

        expect('it should have failed').toBe("but it didn't");
    } catch (err) {
        expect(true).toBe(true);
    }
});

test('config load fails on missing config that must exist', () => {
    const app = require('@claw/app').default;

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    try {
        app.config.load('my-not-here-file.yml', true);
        expect('it should have failed').toBe("but it didn't");
    } catch (err) {
        expect(err).toMatch(/can't load config file: 'my-not-here-file.yml'/);
    }
});

test('load from custom yml', () => {
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

    const [config, configFile] = app.config.load('cla-worker-foo.yml');

    mock.restore();

    expect(config).toMatchObject({ url: 'foobar' });
    expect(configFile).toMatch(/cla-worker-foo.yml$/);
});

test('save to preexisting custom yml', () => {
    const app = require('@claw/app').default;

    mock({
        'cla-workerFOO.yml': '{ url: "foobarbar" }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            config: 'cla-workerFOO.yml'
        }
    });

    const [configFile] = app.config.save({ foobar: 123 });

    const yaml = fs.readFileSync('cla-workerFOO.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        url: 'foobarbar',
        foobar: 123,
        registrations: []
    });

    expect(configFile).toMatch(/cla-workerFOO.yml$/);
});

test('save registration to new default yml', () => {
    const app = require('@claw/app').default;

    const dir = process.cwd();
    mock({
        [dir]: {}
    });

    app.build({
        argv: {}
    });

    const [configFile] = app.config.save({
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

test('save registration to existing default yml', () => {
    const app = require('@claw/app').default;

    const dir = process.cwd();

    mock({
        [`${dir}/cla-worker.yml`]: '{ url: "quzfoo" }'
    });

    app.build({
        argv: {}
    });

    const [configFile] = app.config.save({
        registrations: [{ id: 'quz', token: 'bar' }]
    });

    const yaml = fs.readFileSync('cla-worker.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        url: 'quzfoo',
        registrations: [{ id: 'quz', token: 'bar' }]
    });

    expect(configFile).toMatch(/cla-worker.yml$/);
});

test('save registration to new custom yml with registrations', () => {
    const app = require('@claw/app').default;

    mock({ '/my/path': {} });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            save: true,
            config: '/my/path/my.yml'
        }
    });

    const [configFile] = app.config.save({
        registrations: [{ id: '123', token: 'bar' }]
    });

    const yaml = fs.readFileSync('/my/path/my.yml', 'utf8');
    const data = YAML.safeLoad(yaml);

    mock.restore();

    expect(data).toMatchObject({
        registrations: [{ id: '123', token: 'bar' }]
    });

    expect(configFile).toMatch(/\/my\/path\/my.yml$/);
});

test('save registration to existing yml with registrations', () => {
    const app = require('@claw/app').default;

    mock({
        'cla-worker.yml':
            '{ registrations: [{id: "firstid", token: "firsttoken"}] }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd'
        }
    });

    const [configFile] = app.config.save({
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
