import app from '@claw/app';
import PubSub from '@claw/pubsub';
import Dispatcher from '@claw/Dispatcher';

export const runner = async () => {
    const { id, url, token, tags, envs } = app.config;

    try {
        if (!token) {
            app.warn(`no token detected for workerId=${id}.`);
            app.warn(
                `register your worker first with "cla-worker register" to get an id/token pair or set it with --token [your_token]`
            );
        }

        const pubsub = new PubSub({
            id,
            token,
            tags,
            envs,
            baseURL: url
        });

        app.on('exit', async () => {
            app.info('closing connection to Clarive server...');
            await pubsub.close();
            app.milestone('worker shutdown completed!');
        });

        try {
            await pubsub.connect();
        } catch (err) {
            if (err.status === 401) {
                app.error(
                    `could not connect to server: worker id=${id} is not authorized. Have you ran "cla-worker register"?`
                );
            } else {
                app.error(
                    `could not connect to server: ${err.status ||
                        ''} ${err.message || ''}: ${err.warning ||
                        'server not available or server address not found'}`
                );
            }
            process.exit(1);
        }

        const disposePubSub = pubsub.subscribe(
            [
                'worker.put_file',
                'worker.get_file',
                'worker.exec',
                'worker.eval',
                'worker.ready',
                'worker.shutdown',
                'worker.file_exists',
                'worker.capable'
            ],
            async (eventName, eventData, eventFind, oid) => {
                app.debug('got message:', eventName, eventData);

                try {
                    const dispatcher = new Dispatcher(
                        pubsub,
                        app.config,
                        eventName,
                        oid,
                        eventData
                    );

                    await dispatcher.process();
                } catch (error) {
                    app.error(
                        `error processing message ${eventName} (${oid}): ${error}`
                    );
                }
            },
            err => {
                app.debug(err);

                const msg = err.message
                    ? `${err.message} (code: ${err.status})`
                    : 'server not available';
                app.error(
                    `connection error to server ${pubsub.baseURL}: `,
                    msg
                );
            }
        );

        app.on('exit', () => {
            disposePubSub();
        });
    } catch (err) {
        app.debug(err);
        app.fail('command %s: %s', app.commandName, err);
    }
};
