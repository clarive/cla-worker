import * as types from '@claw/types';
import * as mock from 'mock-fs';

afterEach(mock.restore);

test('build app', () => {
    const app = require('@claw/app').default;
    mock();

    app.build({ argv: { _: ['testcmd'], $0: 'testcmd' } });

    expect(app).toBeDefined();
});

test('check modules', () => {
    expect(types).toBeDefined();
});

test('app args are stronger than config', () => {
    const app = require('@claw/app').default;

    mock({
        'myconfig.yml': '{ id: "333", token: "444" }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            config: 'myconfig.yml',
            id: '123',
            token: '456'
        }
    });

    expect(app.config.data).toEqual(
        expect.objectContaining({ id: '123', token: '456' })
    );
});

test('app arg id loads token from config registration', () => {
    const app = require('@claw/app').default;

    mock({
        'myconfig.yml': '{ registrations: [ { id: "123", token: "456" } ] }'
    });

    app.build({
        argv: { config: 'myconfig.yml', id: '123' }
    });

    expect(app.config.data).toEqual(
        expect.objectContaining({ id: '123', token: '456' })
    );
});

test('app arg token takes precedence over token from config', () => {
    const app = require('@claw/app').default;
    mock();

    mock({
        'myconfig.yml': '{ registrations: [ { id: "123", token: "456" } ] }'
    });

    app.build({
        argv: {
            _: ['testcmd'],
            $0: 'testcmd',
            config: 'myconfig.yml',
            id: '123',
            token: '777'
        }
    });

    expect(app.config.data).toEqual(
        expect.objectContaining({ id: '123', token: '777' })
    );
});
