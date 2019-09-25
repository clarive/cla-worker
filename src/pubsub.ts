import app from '@claw/app';
import * as EventSource from '@claw/util/eventsource';
const axios = require('axios');
const generate = require('shortid');
import { Writable, Readable } from 'stream';
import { origin, workerid, sleep } from '@claw/common';
import { URL, URLSearchParams } from 'url';

import axios from 'axios';

export default class PubSub {
    id: string;
    baseURL: string;
    url: string;
    username: string;
    connected: boolean;
    errorHandler: any;
    subscriptions: any;
    ev: EventSource;
    disconnecting: any;
    lastEventID: any;
    token: string;
    tags: string[];
    origin: string;

    constructor(options: any = {}) {
        this.id = options.id || generate();
        this.baseURL = options.baseURL || '';
        this.username = options.username;
        this.token = options.token || '';
        this.tags = options.tags;
        this.origin = options.origin || origin();
        this.connected = false;
        this.errorHandler = null;
        this.subscriptions = {};
    }

    address(path) {
        const { baseURL, id, token, origin, tags, lastEventID } = this;
        const tagStr = Array.isArray(tags) ? tags.join(',') : tags;
        return `${baseURL}${path}?id=${id}&token=${token}&origin=${origin}&oid=${lastEventID}&tags=${tagStr}&version=${app.version}`;
    }

    parseData(data) {
        try {
            return JSON.parse(data);
        } catch (err) {
            console.error('error in pubsub data', err, data);
        }
    }

    hasSubscriptions() {
        const { subscriptions } = this;
        return Object.keys(subscriptions).length > 0;
    }

    connect() {
        return new Promise((resolve, reject) => {
            if (this.connected) return;
            this.connected = true;
            this.url = this.address('/pubsub/events');

            app.debug(`pubsub url=${this.url}`);

            this.ev = new EventSource(this.address('/pubsub/events'), {
                //withCredentials: true,
                https: { rejectUnauthorized: false },
                headers: {}
            });

            this.ev.onmessage = e => this.onmessage(e, false);

            this.ev.onopen = e => {
                app.milestone(
                    `connected to Clarive server ${
                        this.baseURL
                    } with workerId=${this.id}`
                );
                resolve(e);
            };

            this.ev.onerror = e =>
                this.errorHandler != null ? this.errorHandler(e) : reject(e);

            this.ev.addEventListener(
                'oid',
                e => {
                    this.lastEventID = e.data;
                },
                false
            );
        });
    }

    async register(passkey: string) {
        try {
            const response = await axios({
                method: 'POST',
                url: this.address('/pubsub/register'),
                params: { passkey }
            });

            return response.data;
        } catch (error) {
            const { response } = error;
            if (response && response.statusText) {
                throw `error registering worker at ${this.baseURL} ==> ${
                    response.status
                } ${response.statusText}: ${response.data || ''}`;
            } else {
                throw error;
            }
        }
    }

    async unregister() {
        const { token } = this;
        try {
            const response = await axios({
                method: 'POST',
                url: this.address('/pubsub/unregister'),
                params: { token }
            });

            return response.data;
        } catch (error) {
            const { response } = error;
            if (response && response.statusText) {
                throw `error unregistering worker at ${this.baseURL} ==> ${
                    response.status
                } ${response.statusText}: ${response.data || ''}`;
            } else {
                throw error;
            }
        }
    }

    maybeConnect() {
        setTimeout(() => {
            const { disconnecting } = this;
            if (this.hasSubscriptions()) {
                if (disconnecting) clearTimeout(disconnecting);
                this.connect();
            }
        }, 1000);
    }

    onmessage(e, filterOutSameUser = false) {
        const { subscriptions, parseData } = this;
        const data = parseData(e.data);

        app.debug('onmessage=', e);

        if (!data) return;
        if (data.oid) this.lastEventID = data.oid;

        const { events } = data;

        if (events) {
            Object.keys(events).forEach(eventKey => {
                let evData = events[eventKey] || {};

                if (filterOutSameUser === true) {
                    evData = evData.filter(e => e.username !== this.username);
                }

                app.debug(`got ${eventKey}`, evData);

                if (evData.length === 0) return;

                let callbacks = [];

                Object.keys(subscriptions).forEach(key => {
                    const reg = /[\^\$]/.test(key)
                        ? new RegExp(key)
                        : new RegExp('^' + key + '$');

                    const ret = eventKey.replace(reg, '');

                    if (!ret || ret.charAt(0) === '.') {
                        callbacks = callbacks.concat(subscriptions[key]);
                    }
                });

                const find = obj => {
                    let results = evData;

                    Object.keys(obj).forEach(key => {
                        let val = obj[key];

                        val = Array.isArray(val) ? val : [val];
                        results = results.filter(
                            ev => val.indexOf(ev[key]) !== -1
                        );
                    });
                    return results.length ? results : null;
                };

                callbacks.forEach(cb =>
                    evData.forEach(ev => cb(eventKey, ev, find, data.oid))
                );
            });
        }
    }

    subscribe(channel, cb, cbErr) {
        const { subscriptions } = this;

        const channels = Array.isArray(channel) ? channel : [channel];

        channels.forEach(ch => {
            if (typeof ch !== 'string') {
                throw new Error(
                    'channel must be a string or an array of strings'
                );
            }

            if (!subscriptions[ch]) {
                subscriptions[ch] = [];
            }

            subscriptions[ch].push(cb);
        });

        this.errorHandler = cbErr;
        this.maybeConnect();

        return () => {
            this.unsubscribe(channel, cb);
        };
    }

    unsubscribe(channel, func) {
        const { subscriptions } = this;
        const channels = Array.isArray(channel) ? channel : [channel];
        channels.forEach(ch => {
            if (typeof ch !== 'string') {
                throw new Error(
                    'channel must be a string or an array of strings'
                );
            }

            // disposed twice!!
            if (!subscriptions[ch]) {
                return;
            }

            if (!func) {
                delete subscriptions[ch];
            } else {
                const index = subscriptions[ch].indexOf(func);
                if (index !== -1) {
                    subscriptions[ch].splice(index, 1);
                    if (subscriptions[ch].length === 0) {
                        delete subscriptions[ch];
                    }
                }
            }
        });

        this.maybeDisconnect();
    }

    maybeDisconnect() {
        this.disconnecting = setTimeout(() => {
            if (!this.hasSubscriptions()) {
                this.close();
            }
        }, 5000);
    }

    async close() {
        app.debug('closing pubsub event watcher...');

        if (!this.connected) return;
        this.connected = false;

        try {
            if (this.ev) {
                await this.ev.close();
                this.ev = null;
            }
        } catch (err) {
            app.debug('event close request failed');
        }

        app.debug('sending close request to pubsub...');

        try {
            const res = await axios({
                method: 'post',
                url: this.address('/pubsub/close'),
                data: { id: this.id, token: this.token }
            });
        } catch (err) {
            app.debug('pubsub close request failed');
        }

        app.debug('pubsub closed');
    }

    async poll() {
        if (this.hasSubscriptions()) {
            const res = await axios({
                method: 'post',
                url: this.address('/pubsub/poll'),
                data: {
                    channels: [],
                    oid: this.lastEventID
                }
            });
            this.onmessage({ data: res });
            this.connect();
        }
    }

    async publish(key: string, data: any) {
        try {
            let response;
            let maxTries = 10;
            let waitTime = 1000;

            // make it resillient to server restarts, comm failures
            while (maxTries--) {
                try {
                    response = await axios({
                        method: 'POST',
                        url: this.address('/pubsub/publish'),
                        data: { event: key, ...data }
                    });

                    break;
                } catch (err) {
                    if (maxTries > 0) {
                        waitTime = (await sleep(waitTime)) * 2;
                        continue;
                    }
                    throw err;
                }
            }

            app.info(`pubsub published event=${key}`, data);

            return response;
        } catch (error) {
            const { response } = error;

            if (response && response.statusText) {
                throw `error publishing data to ${this.baseURL} ==> ${
                    response.status
                } ${response.statusText}: ${response.data || ''}`;
            } else {
                throw error;
            }
        }
    }

    async pop(key: string, stream: Writable) {
        try {
            const response = await axios({
                method: 'POST',
                url: this.address('/pubsub/pop'),
                params: { key },
                responseType: 'stream'
            });

            response.data.pipe(stream);

            return response;
        } catch (error) {
            const { response } = error;
            if (response && response.statusText) {
                throw `error downloading data ${key} from ${this.baseURL} ==> ${
                    response.status
                } ${response.statusText}: ${response.data || ''}`;
            } else {
                throw error;
            }
        }
    }

    async push(key: string, filename: string, stream: Readable) {
        try {
            const response = await axios({
                method: 'POST',
                maxContentLength: Infinity,
                url: this.address('/pubsub/push'),
                headers: {
                    'Content-Type': 'application/octet-stream'
                },
                params: { key, filename },
                data: stream
            });

            return response;
        } catch (error) {
            const { response } = error;
            if (response && response.statusText) {
                throw `error uploading data ${key} to ${this.baseURL} ==> ${
                    response.status
                } ${response.statusText}: ${response.data || ''}`;
            } else {
                throw error;
            }
        }
    }
}
