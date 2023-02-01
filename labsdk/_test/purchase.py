# -*- coding: utf-8 -*-
#  Copyright (c) 2022 RaptorML authors.
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.


# Here is an example of a complete notebook that implements a simple use case for data coming from both REST APIs and
# streaming data sources, with aggregation of a feature using a streaming data source. The use case is a simple customer
# purchase prediction model, where the input features include the number of purchases made by a customer in the last 10
# hours, as well as the price of the last purchase. The input labels are the amount spent by the customer in the next
# purchase. The model is trained using XGBoost and evaluated using accuracy.

from datetime import datetime
from typing import TypedDict

import pandas as pd

from ..raptor import data_source, Context, feature, aggregation, AggregationFunction, freshness, model, \
    TrainingContext, StreamingConfig


# Data source for the purchase history data
@data_source(
    training_data=pd.read_parquet(
        'https://gist.github.com/AlmogBaku/a1b331615eaf1284432d2eecc5fe60bc/raw/purchases.parquet'),
    keys=['id', 'customer_id'],
    timestamp='purchase_at',
    production_config=StreamingConfig(kind='kafka'),
)
class Purchase(TypedDict):
    purchase_at: datetime
    customer_id: str
    amount: float
    price: float


# Feature that counts the number of purchases made by a customer in the last 10 hours
@feature(keys='customer_id', data_source=Purchase)
@aggregation(function=AggregationFunction.Count, over='10h', granularity='1h')
def purchases_10h(this_row: Purchase, ctx: Context) -> int:
    """number of purchases made by a customer in the last 10 hours"""
    return 1


# Feature that gets the price of the last purchase made by a customer
@feature(keys='customer_id', data_source=Purchase)
def last_price(this_row: Purchase, ctx: Context) -> float:
    """price of the last purchase made by a customer"""
    return this_row['price']


# Model that predicts the amount spent by a customer in the next purchase
@model(
    keys=['customer_id'],
    input_features=['purchases_10h', 'last_price'],
    input_labels=['amount'],
    model_framework='xgboost',
    model_server='sagemaker-ack',
)
@freshness(target='1h', invalid_after='100h')
def purchase_prediction(ctx: TrainingContext) -> float:
    from xgboost import XGBRegressor
    from sklearn.model_selection import train_test_split

    df = ctx.features_and_labels()
    X = df[ctx.input_features]
    y = df[ctx.input_labels]

    # Split the data into training and testing sets
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=0)

    # Initialize an XGBoost model
    xgb_model = XGBRegressor()

    # Fit the model to the training data
    xgb_model.fit(X_train, y_train)

    # Evaluate the model on the testing data
    accuracy = xgb_model.score(X_test, y_test)

    # Make sure the model has a minimum accuracy of 0.6
    if accuracy < 0.6:
        raise Exception('Accuracy is below 0.7')

    return xgb_model
