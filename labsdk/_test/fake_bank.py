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

import datetime
import random

import numpy as np
import pandas as pd

# Define the number of transactions and customers
num_transactions = 3000
num_customers = 10

# Create a list of customer IDs
customer_ids = [f'customer_{i}' for i in range(num_customers)]

# Generate a list of transaction amounts
transaction_amounts = [random.uniform(10, 500) for i in range(num_transactions)]

# Generate a list of transaction dates
start_date = datetime.datetime(2023, 1, 1)
end_date = datetime.datetime(2023, 1, 5)
date_range = pd.date_range(start_date, end_date, freq='min')
transaction_dates = np.random.choice(date_range, num_transactions)

# Create a dataframe to store the transactions
transactions_df = pd.DataFrame({
    'customer_id': np.random.choice(customer_ids, num_transactions),
    'amount': transaction_amounts,
    'timestamp': transaction_dates
})

# Raptor
from typing_extensions import TypedDict
from labsdk.raptor import data_source, Context, feature, aggregation, AggregationFunction, freshness, model, \
    TrainingContext, StreamingConfig


@data_source(
    training_data=transactions_df,
    keys=['customer_id'],
    production_config=StreamingConfig()
)
class BankTransaction(TypedDict):
    customer_id: int
    amount: float
    timestamp: str


# Define features
@feature(keys='customer_id', data_source=BankTransaction)
@aggregation(function=AggregationFunction.Sum, over='10h', granularity='1h')
def total_spend(this_row: BankTransaction, ctx: Context) -> float:
    """total spend by a customer in the last hour"""
    return this_row['amount']


@feature(keys='customer_id', data_source=BankTransaction)
@freshness(target='5h', invalid_after='1d')
def amount(this_row: BankTransaction, ctx: Context) -> float:
    """total spend by a customer in the last hour"""
    return this_row['amount']


# Train the model
@model(
    keys=['customer_id'],
    input_features=['total_spend+sum'],
    input_labels=[amount],
    model_framework='sklearn',
    model_server='sagemaker-ack',
)
@freshness(target='1h', invalid_after='100h')
def amount_prediction(ctx: TrainingContext):
    from sklearn.linear_model import LinearRegression

    df = ctx.features_and_labels()

    trainer = LinearRegression()
    trainer.fit(df[ctx.input_features], df[ctx.input_labels])

    return trainer


amount_prediction.export()
