export const FETCH_USER_BEGIN = 'FETCH_USER_BEGIN';
export const FETCH_USER_SUCCESS = 'FETCH_USER_SUCCESS';
export const FETCH_USER_ERROR = 'FETCH_USER_ERROR';

export const fetchUserBegin = () => ({
  type: FETCH_USER_BEGIN
});

export const fetchUserSuccess = user => ({
  type: FETCH_USER_SUCCESS,
  payload: { user }
});

export const fetchUserError = error => ({
  type: FETCH_USER_ERROR,
  payload: { error }
});

export const fetchUser = (userId) => dispatch => {
  console.log("fetchUser()");
  dispatch(fetchUserBegin());
  fetch(`/api/user/${userId}`)
    .then(handleErrors)
    .then(res => res.json())
    .then(json => {
      console.log(json);
      dispatch(fetchUserSuccess(json));
      return json;
    })
    .catch(error => { dispatch(fetchUserError(error)) });
}

// Handle HTTP errors since fetch won't.
function handleErrors(response) {
  console.log(response);
  if (!response.ok) {
    throw Error(response.statusText);
  }
  return response;
}