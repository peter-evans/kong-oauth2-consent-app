# OAuth 2.0 Consent Application for use with Kong
[![Go Report Card](https://goreportcard.com/badge/github.com/peter-evans/kong-oauth2-consent-app)](https://goreportcard.com/report/github.com/peter-evans/kong-oauth2-consent-app)

This is a sample consent application demonstrating the OAuth 2.0 Authorization Code Grant flow with [Kong, the microservice API gateway](https://konghq.com/).

## OAuth 2.0 Authorization Code Grant Sequence

In the traditional Authorization Code Grant flow the "Authorization Server" generally handles both obtaining consent from the user _and_ serving authorization codes and the subsequent access/refresh tokens.
With Kong in the picture, the roles need to be split into two. Kong can serve authorization codes and access/refresh tokens but it has no functionality to authenticate the user and obtain their consent.

The following sequence diagram shows how the role of "Authorization Server" is split between an application to obtain consent from the user and Kong as the OAuth 2.0 provider.

![Authorization Code Grant](resources/authorization-code-grant.png?raw=true)

## Usage

#### Configure the environment

1. Install the sample consent app.

   ```bash
   $ go get -u -d github.com/peter-evans/kong-oauth2-consent-app
   $ cd $GOPATH/src/github.com/peter-evans/kong-oauth2-consent-app
   ```
2. If you don't have a Kong instance available you can run a Kong stack locally with the [docker-compose configuration](docker-compose.yml) provided.

   ```bash
   $ docker-compose up -d
   ```
3. Add a test service and route.

   ```bash
   $ curl -i -X POST \
     --url http://localhost:8001/services/ \
     --data 'name=test-service' \
     --data 'url=http://mockbin.org'
   ```
   ```bash
   $ curl -i -X POST \
     --url http://localhost:8001/services/test-service/routes \
     --data 'paths[]=/myapi'
   ```
4. Configure the OAuth 2.0 plugin on the service and make a note of the `provision_key` in the response.

   ```bash
   $ curl -i -X POST \
     --url http://localhost:8001/services/test-service/plugins \
     --data 'name=oauth2' \
     --data 'config.scopes=email,phone,address' \
     --data 'config.mandatory_scope=true' \
     --data 'config.enable_authorization_code=true'
   ```
5. Create a consumer representing the owner of the client application.

   ```bash
   $ curl -i -X POST \
     --url http://localhost:8001/consumers \
     --data 'username=testclient'
   ```
6. Register OAuth 2.0 credentials representing the client application and make a note of the `client_id` and `client_secret` in the response.

   ```bash
   $ curl -i -X POST \
     --url http://localhost:8001/consumers/testclient/oauth2 \
     --data 'name=Test%20Client%20Application' \
     --data 'redirect_uri=http://some-domain/endpoint/'
   ```

#### Running the consent application

1. Edit [run.sh](run.sh) and update with the `provision_key` and `client_id` noted earlier.
   The `client_id` is configured in this way for demonstration purposes only.
   In production this value should be passed to the consent application by the client.
   
   ```bash
   export PROVISION_KEY="XXX"
   export DEMO_CLIENT_ID="XXX"
   ```
2. Execute run.sh

   ```bash
   $ ./run.sh
   ```
3. Browse to [http://localhost:8080](http://localhost:8080) where you can begin the OAuth 2.0 authorization code grant flow.

   ![Authorize Application](resources/authorize-application.png?raw=true)
   
   After authorizing the client application a `redirect_uri` will be displayed.
   In production systems the user should be immediately redirected back to the client application via this URI.
   The URI should contain the authorization `code` as a querystring parameter.
   
   ```
   redirect_uri: http://some-domain/endpoint/?code=JJxhzunaoilSXgTpl24qjNM8hZqttAn5
   ```

#### Obtaining OAuth 2.0 tokens

The client application can now use the authorization code to obtain an access token and refresh token directly from Kong.
Test this using the `client_id`, `client_secret` and authorization `code` noted earlier.

```bash
$ curl -i -X POST \
  --url https://localhost:8443/myapi/oauth2/token \
  --data 'grant_type=authorization_code' \
  --data 'client_id=XXX' \
  --data 'client_secret=XXX' \
  --data 'code=XXX' --insecure
```

When the access token expires a new token can be obtained using the refresh token.

```bash
$ curl -i -X POST \
  --url https://localhost:8443/myapi/oauth2/token \
  --data 'grant_type=refresh_token' \
  --data 'client_id=XXX' \
  --data 'client_secret=XXX' \
  --data 'refresh_token=XXX' --insecure
```

## Reference

- [OAuth 2.0 RFC: Authorization Code Grant](https://tools.ietf.org/html/rfc6749#section-4.1)
- [Kong's OAuth 2.0 Authentication Plugin](https://docs.konghq.com/plugins/oauth2-authentication/)

## License

MIT License - see the [LICENSE](LICENSE) file for details
