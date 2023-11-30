import aws_cdk as core
import aws_cdk.assertions as assertions

from analytiikka_vesi.analytiikka_vesi_stack import AnalytiikkaVesiStack

# example tests. To run these tests, uncomment this file along with the example
# resource in analytiikka_vesi/analytiikka_vesi_stack.py
def test_sqs_queue_created():
    app = core.App()
    stack = AnalytiikkaVesiStack(app, "analytiikka-vesi")
    template = assertions.Template.from_stack(stack)

#     template.has_resource_properties("AWS::SQS::Queue", {
#         "VisibilityTimeout": 300
#     })
