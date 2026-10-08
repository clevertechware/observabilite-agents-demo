use axum::{
    extract::{Path, Query, State},
    Json,
};
use serde::{Deserialize, Serialize};

use crate::{catalog, error::ApiError, faults::Faults};

#[derive(Deserialize)]
pub struct PriceQuery {
    quantity: u32,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Price {
    sku: String,
    quantity: u32,
    unit_price: f64,
    total: f64,
}

pub async fn get_price(
    State(faults): State<Faults>,
    Path(sku): Path<String>,
    Query(query): Query<PriceQuery>,
) -> Result<Json<Price>, ApiError> {
    faults.apply()?;
    if query.quantity == 0 {
        return Err(ApiError::InvalidQuantity);
    }
    match catalog::unit_price(&sku, query.quantity) {
        Some(unit_price) => {
            tracing::info!(%sku, quantity = query.quantity, unit_price, "price computed");
            Ok(Json(Price {
                sku,
                quantity: query.quantity,
                unit_price,
                total: catalog::total(unit_price, query.quantity),
            }))
        }
        None => {
            tracing::warn!(%sku, "unknown sku");
            Err(ApiError::UnknownSku)
        }
    }
}
