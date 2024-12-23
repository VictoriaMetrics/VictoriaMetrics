from datadog_lambda.metric import lambda_metric

def lambda_handler(event, context):
    lambda_metric(
        metric_name='coffee_house.order_value',
        value=12.45,
        tags=['product:latte', 'order:online']
    )

    print('Hello, world!')

    return {
        'statusCode': 200,
        'body': 'Hello from serverless!'
    }
