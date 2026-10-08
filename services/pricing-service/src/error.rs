use axum::{
    http::StatusCode,
    response::{IntoResponse, Response},
};

#[derive(Debug, thiserror::Error)]
pub enum ApiError {
    #[error("unknown sku")]
    UnknownSku,
    #[error("quantity must be at least 1")]
    InvalidQuantity,
    #[error("injected failure")]
    InjectedFailure,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let status = match self {
            ApiError::UnknownSku => StatusCode::NOT_FOUND,
            ApiError::InvalidQuantity => StatusCode::BAD_REQUEST,
            ApiError::InjectedFailure => StatusCode::INTERNAL_SERVER_ERROR,
        };
        (status, self.to_string()).into_response()
    }
}
